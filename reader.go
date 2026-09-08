package gws

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	"github.com/lxzan/gws/internal"
)

// errOpenPanic OnOpen回调panic被Recovery拦截后返回的错误, 用于驱动ReadLoop按读错误路径关闭连接
var errOpenPanic = errors.New("gws: panic in OnOpen")

// checkMask 检查掩码设置是否符合 RFC6455 协议
func (c *Conn) checkMask(enabled bool) error {
	// RFC6455: 所有从客户端发送到服务器的帧都必须设置掩码位为 1
	if (c.isServer && !enabled) || (!c.isServer && enabled) {
		return internal.CloseProtocolError
	}
	return nil
}

// readControl 读取控制帧
func (c *Conn) readControl() error {
	// RFC6455 §5.5: 控制帧不允许设置任何RSV位
	if c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3() {
		return internal.CloseProtocolError
	}

	// RFC6455: 控制帧本身不能被分片
	if !c.fh.GetFIN() {
		return internal.CloseProtocolError
	}

	// RFC6455: 所有控制帧的有效载荷长度必须为 125 字节或更少
	var n = c.fh.GetLengthCode()
	if n > internal.ThresholdV1 {
		return internal.CloseProtocolError
	}

	// 不回收小块 buffer, 控制帧一般 payload 长度为 0
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
		return c.dispatchControl(OpcodePing, payload, nil)
	case OpcodePong:
		return c.dispatchControl(OpcodePong, payload, nil)
	case OpcodeCloseConnection:
		return c.emitClose(bytes.NewBuffer(payload))
	default:
		var err = fmt.Errorf("gws: unexpected opcode %d", opcode)
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

// readMessage 读取消息, 组装出完整的消息后派发给 OnMessage 回调
func (c *Conn) readMessage() error {
	msg, err := c.readFrame()
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	return c.emitReadMessage(msg)
}

func (c *Conn) emitReadMessage(msg *Message) error {
	if err := c.emitMessage(msg); err != nil {
		_ = msg.Close()
		return err
	}
	return nil
}

// readFrame 读取一帧数据.
// 如果这一帧拼装出了一条完整的消息(未分片的数据帧, 或者分片消息的最后一帧), 则返回该消息;
// 如果处理的是控制帧或者未结束的分片帧, 则返回 (nil, nil).
func (c *Conn) readFrame() (msg *Message, err error) {
	// 解析帧头并获取内容长度
	contentLength, err := c.fh.Parse(c.br)
	if err != nil {
		return nil, err
	}
	if contentLength > c.config.ReadMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}

	// RSV1, RSV2, RSV3: 每个占 1 位, 必须为 0 除非协商的扩展定义了非零值的含义.
	// 如果接收到非零值且没有协商的扩展定义该非零值的含义, 接收端点必须关闭 WebSocket 连接.
	// 控制帧的RSV校验在readControl入口处执行;
	// RFC7692: continuation帧不允许设置RSV1(压缩标记只允许出现在消息的首个分片).
	var opcode = c.fh.GetOpcode()
	if opcode == OpcodeContinuation {
		if c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3() {
			return nil, internal.CloseProtocolError
		}
	} else if opcode.isDataFrame() && (c.fh.GetRSV2() || c.fh.GetRSV3() || (!c.pd.Enabled && c.fh.GetRSV1())) {
		return nil, internal.CloseProtocolError
	}

	maskEnabled := c.fh.GetMask()
	if err := c.checkMask(maskEnabled); err != nil {
		return nil, err
	}

	var compressed = c.pd.Enabled && c.fh.GetRSV1()
	if !opcode.isDataFrame() {
		return nil, c.readControl()
	}

	var fin = c.fh.GetFIN()
	var buf = binaryPool.Get(contentLength + len(flateTail))
	var p = buf.Bytes()[:contentLength]

	// buf 默认在函数返回时回收, 除非其所有权被转移给了返回的 Message(此时由调用方负责回收)
	var recycle = true
	defer func() {
		if recycle {
			binaryPool.Put(buf)
		}
	}()

	if err := internal.ReadN(c.br, p); err != nil {
		return nil, err
	}
	if maskEnabled {
		internal.MaskXOR(p, c.fh.GetMaskKey())
	}

	if opcode != OpcodeContinuation && c.continuationFrame.initialized {
		return nil, internal.CloseProtocolError
	}

	if fin && opcode != OpcodeContinuation {
		*(*[]byte)(unsafe.Pointer(buf)) = p
		recycle = false
		msg = messagePool.Get()
		msg.Opcode = opcode
		msg.Data = buf
		msg.compressed = compressed
		return msg, nil
	}

	// 处理分片消息
	if !fin && opcode != OpcodeContinuation {
		c.continuationFrame.initialized = true
		c.continuationFrame.compressed = compressed
		c.continuationFrame.opcode = opcode
		c.continuationFrame.buffer = bytes.NewBuffer(make([]byte, 0, contentLength))
	}
	if !c.continuationFrame.initialized {
		return nil, internal.CloseProtocolError
	}

	c.continuationFrame.buffer.Write(p)
	if c.continuationFrame.buffer.Len() > c.config.ReadMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}
	if !fin {
		return nil, nil
	}

	msg = messagePool.Get()
	msg.Opcode = c.continuationFrame.opcode
	msg.Data = c.continuationFrame.buffer
	msg.compressed = c.continuationFrame.compressed
	c.continuationFrame.reset()
	return msg, nil
}

func (c *Conn) readDataFrameHeader(expectContinuation bool) (*frameHeader, int, error) {
	for {
		contentLength, err := c.fh.Parse(c.br)
		if err != nil {
			return nil, 0, err
		}
		if contentLength > c.config.ReadMaxPayloadSize {
			return nil, 0, internal.CloseMessageTooLarge
		}
		if !c.pd.Enabled && (c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3()) {
			return nil, 0, internal.CloseProtocolError
		}
		maskEnabled := c.fh.GetMask()
		if err := c.checkMask(maskEnabled); err != nil {
			return nil, 0, err
		}
		opcode := c.fh.GetOpcode()
		if !opcode.isDataFrame() {
			if err := c.readControl(); err != nil {
				return nil, 0, err
			}
			continue
		}
		if expectContinuation {
			if opcode != OpcodeContinuation || c.fh.GetRSV1() {
				return nil, 0, internal.CloseProtocolError
			}
		} else {
			if opcode == OpcodeContinuation {
				return nil, 0, internal.CloseProtocolError
			}
		}
		if c.fh.GetRSV2() || c.fh.GetRSV3() {
			return nil, 0, internal.CloseProtocolError
		}
		return &c.fh, contentLength, nil
	}
}

// dispatchOpen 分发连接建立事件并进行异常恢复.
// OnOpen回调如果发生 panic, 由 Config.Recovery 兜底, 并返回 errOpenPanic,
// 使 ReadLoop 按读错误路径关闭当前连接, 避免 panic 击穿进程.
func (c *Conn) dispatchOpen() (err error) {
	var done = false
	defer func() {
		if !done {
			err = errOpenPanic
		}
	}()
	defer c.config.Recovery(c.config.Logger)
	c.handler.OnOpen(c)
	done = true
	return nil
}

// dispatchMessage 分发消息和异常恢复
func (c *Conn) dispatchMessage(msg *Message) error {
	defer c.config.Recovery(c.config.Logger)
	c.handler.OnMessage(c, msg)
	return nil
}

// dispatchControl 分发控制帧事件并进行异常恢复.
// 控制帧(Ping/Pong/Close)的回调如果发生 panic, 不应直接导致 ReadLoop 崩溃;
// 因此这里统一通过 Config.Recovery 进行兜底.
func (c *Conn) dispatchControl(opcode Opcode, payload []byte, err error) error {
	defer c.config.Recovery(c.config.Logger)
	switch opcode {
	case OpcodePing:
		c.handler.OnPing(c, payload)
	case OpcodePong:
		c.handler.OnPong(c, payload)
	case OpcodeCloseConnection:
		c.handler.OnClose(c, err)
	}
	return nil
}

// decompressMessage 解压消息并更新解压字典
func (c *Conn) decompressMessage(msg *Message) error {
	if msg.compressed {
		var rawBuf = msg.Data
		dst, err := c.deflater.Decompress(rawBuf, c.dpsWindow.dict)
		binaryPool.Put(rawBuf)
		if err != nil {
			msg.Data = nil
			// 解压超限等已携带语义化状态码(如1009)的错误直接透传, 避免被包裹为1011
			if _, ok := err.(internal.StatusCode); ok {
				return err
			}
			return internal.NewError(internal.CloseInternalErr, err)
		}
		msg.Data = dst
		_, _ = c.dpsWindow.Write(msg.Bytes())
	}
	return nil
}

// processMessage 处理消息: 解压并校验编码
func (c *Conn) processMessage(msg *Message) error {
	if err := c.decompressMessage(msg); err != nil {
		return err
	}
	if !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(msg.Opcode), msg.Bytes()) {
		return internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}
	return nil
}

// emitMessage 发射消息事件
func (c *Conn) emitMessage(msg *Message) error {
	if err := c.processMessage(msg); err != nil {
		return err
	}
	if c.config.ParallelEnabled {
		return c.readQueue.Go(msg, c.dispatchMessage)
	}
	return c.dispatchMessage(msg)
}
