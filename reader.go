package gws

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/lxzan/gws/internal"
)

// 检查掩码设置是否符合 RFC6455 协议。
// Checks if the mask setting complies with the RFC6455 protocol.
func (c *Conn) checkMask(enabled bool) error {
	// RFC6455: 所有从客户端发送到服务器的帧都必须设置掩码位为 1。
	// RFC6455: All frames sent from client to server must have the mask bit set to 1.
	if (c.isServer && !enabled) || (!c.isServer && enabled) {
		return internal.CloseProtocolError
	}
	return nil
}

// 读取控制帧
// Reads a control frame
func (c *Conn) readControl() error {
	// RFC6455: 控制帧本身不能被分片。
	// RFC6455: Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return internal.CloseProtocolError
	}

	// RFC6455: 所有控制帧的有效载荷长度必须为 125 字节或更少，并且不能被分片。
	// RFC6455: All control frames MUST have a payload length of 125 bytes or fewer and MUST NOT be fragmented.
	var n = c.fh.GetLengthCode()
	if n > internal.ThresholdV1 {
		return internal.CloseProtocolError
	}

	// 不回收小块 buffer，控制帧一般 payload 长度为 0
	// Do not recycle small buffers, control frames generally have a payload length of 0
	var payload []byte
	if n > 0 {
		payload = make([]byte, n)
		if err := internal.ReadN(c.br, payload); err != nil {
			return err
		}
		if maskEnabled := c.fh.GetMask(); maskEnabled {
			internal.MaskXOR(payload, c.fh.GetMaskKey())
		}
	}

	var opcode = c.fh.GetOpcode()
	switch opcode {
	case OpcodePing:
		c.handler.OnPing(c, payload)
		return nil
	case OpcodePong:
		c.handler.OnPong(c, payload)
		return nil
	case OpcodeCloseConnection:
		return c.emitClose(bytes.NewBuffer(payload))
	default:
		var err = fmt.Errorf("gws: unexpected opcode %d", opcode)
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

// 读取消息
// Reads a message
func (c *Conn) readMessage() error {
	// 解析帧头并获取内容长度
	// Parse the frame header and get the content length
	contentLength, err := c.fh.Parse(c.br)
	if err != nil {
		return err
	}
	if contentLength > c.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	// RSV1, RSV2, RSV3: 每个占 1 位
	// 必须为 0，除非协商的扩展定义了非零值的含义。
	// 如果接收到非零值且没有协商的扩展定义该非零值的含义，接收端点必须关闭 WebSocket 连接。
	// RSV1, RSV2, RSV3: 1 bit each
	// MUST be 0 unless an extension is negotiated that defines meanings for non-zero values.
	// If a nonzero value is received and none of the negotiated extensions defines the meaning of such a nonzero value,
	// the receiving endpoint MUST _Fail the WebSocket Connection_.
	if !c.pd.Enabled && (c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3()) {
		return internal.CloseProtocolError
	}

	maskEnabled := c.fh.GetMask()
	if err := c.checkMask(maskEnabled); err != nil {
		return err
	}

	var opcode = c.fh.GetOpcode()
	var compressed = c.pd.Enabled && c.fh.GetRSV1()
	if !opcode.isDataFrame() {
		return c.readControl()
	}

	var fin = c.fh.GetFIN()
	var buf = binaryPool.Get(contentLength + len(flateTail))
	var p = buf.Bytes()[:contentLength]
	var closer = Message{Data: buf}
	defer closer.Close()

	if err := internal.ReadN(c.br, p); err != nil {
		return err
	}
	if maskEnabled {
		internal.MaskXOR(p, c.fh.GetMaskKey())
	}

	if opcode != OpcodeContinuation && c.continuationFrame.initialized {
		return internal.CloseProtocolError
	}

	if fin && opcode != OpcodeContinuation {
		*(*[]byte)(unsafe.Pointer(buf)) = p
		if !compressed {
			closer.Data = nil
		}
		return c.emitMessage(&Message{Opcode: opcode, Data: buf, compressed: compressed})
	}

	// 处理分片消息
	// processing segmented messages
	if !fin && opcode != OpcodeContinuation {
		c.continuationFrame.initialized = true
		c.continuationFrame.compressed = compressed
		c.continuationFrame.opcode = opcode
		c.continuationFrame.buffer = bytes.NewBuffer(make([]byte, 0, contentLength))
	}
	if !c.continuationFrame.initialized {
		return internal.CloseProtocolError
	}

	c.continuationFrame.buffer.Write(p)
	if c.continuationFrame.buffer.Len() > c.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}
	if !fin {
		return nil
	}

	msg := &Message{Opcode: c.continuationFrame.opcode, Data: c.continuationFrame.buffer, compressed: c.continuationFrame.compressed}
	c.continuationFrame.reset()
	return c.emitMessage(msg)
}

// 分发消息和异常恢复
// Dispatch message & Recovery
func (c *Conn) dispatch(msg *Message) error {
	defer c.config.Recovery(c.config.Logger)
	c.handler.OnMessage(c, msg)
	return nil
}

// 发射消息事件
// Emit onmessage event
func (c *Conn) emitMessage(msg *Message) (err error) {
	if msg.compressed {
		msg.Data, err = c.deflater.Decompress(msg.Data, c.dpsWindow.dict)
		if err != nil {
			return internal.NewError(internal.CloseInternalErr, err)
		}
		_, _ = c.dpsWindow.Write(msg.Bytes())
	}
	if !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(msg.Opcode), msg.Bytes()) {
		return internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}
	if c.config.ParallelEnabled {
		return c.readQueue.Go(msg, c.dispatch)
	}
	return c.dispatch(msg)
}
