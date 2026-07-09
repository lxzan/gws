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

// 读取消息, 组装出完整的消息后派发给 OnMessage 回调
// Reads a message, dispatching it to the OnMessage callback once fully assembled
func (c *Conn) readMessage() error {
	msg, err := c.readFrame()
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	return c.emitMessage(msg)
}

// 读取一帧数据.
// 如果这一帧拼装出了一条完整的消息(未分片的数据帧, 或者分片消息的最后一帧), 则返回该消息;
// 如果处理的是控制帧或者未结束的分片帧, 则返回 (nil, nil).
// Reads a single frame.
// If this frame completes a message (an unfragmented data frame, or the final frame of a
// fragmented message), the message is returned; if a control frame or a non-final fragment
// was processed, (nil, nil) is returned.
func (c *Conn) readFrame() (msg *Message, err error) {
	// 解析帧头并获取内容长度
	// Parse the frame header and get the content length
	contentLength, err := c.fh.Parse(c.br)
	if err != nil {
		return nil, err
	}
	if contentLength > c.config.ReadMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}

	// RSV1, RSV2, RSV3: 每个占 1 位
	// 必须为 0，除非协商的扩展定义了非零值的含义。
	// 如果接收到非零值且没有协商的扩展定义该非零值的含义，接收端点必须关闭 WebSocket 连接。
	// RSV1, RSV2, RSV3: 1 bit each
	// MUST be 0 unless an extension is negotiated that defines meanings for non-zero values.
	// If a nonzero value is received and none of the negotiated extensions defines the meaning of such a nonzero value,
	// the receiving endpoint MUST _Fail the WebSocket Connection_.
	if !c.pd.Enabled && (c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3()) {
		return nil, internal.CloseProtocolError
	}

	maskEnabled := c.fh.GetMask()
	if err := c.checkMask(maskEnabled); err != nil {
		return nil, err
	}

	var opcode = c.fh.GetOpcode()
	var compressed = c.pd.Enabled && c.fh.GetRSV1()
	if !opcode.isDataFrame() {
		return nil, c.readControl()
	}

	var fin = c.fh.GetFIN()
	var buf = binaryPool.Get(contentLength + len(flateTail))
	var p = buf.Bytes()[:contentLength]

	// buf 默认在函数返回时回收, 除非其所有权被转移给了返回的 Message(此时由调用方负责回收)
	// buf is recycled on return by default, unless its ownership is transferred to the
	// returned Message (in which case the caller is responsible for recycling it)
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
		return &Message{Opcode: opcode, Data: buf, compressed: compressed}, nil
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
		return nil, internal.CloseProtocolError
	}

	c.continuationFrame.buffer.Write(p)
	if c.continuationFrame.buffer.Len() > c.config.ReadMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}
	if !fin {
		return nil, nil
	}

	msg = &Message{Opcode: c.continuationFrame.opcode, Data: c.continuationFrame.buffer, compressed: c.continuationFrame.compressed}
	c.continuationFrame.reset()
	return msg, nil
}

// readStreamChunk 为 Conn.Read 读取更多可供消费的数据.
//
// 如果这一帧只是一个控制帧(ping/pong/close, 已经完成派发), 返回 (nil, nil), 调用方需要再次调用以获取真正
// 的数据.
//
// 未压缩的消息按帧直接交付, 读到一帧就返回一帧的数据, 不必等待分片消息的后续帧到达; 压缩的消息由于需要
// 对整条消息的字节流一起做 inflate, 会累积消息的所有分片, 读到最后一帧后整体解压再交付. 是否压缩、消息的
// opcode 等状态复用 c.continuationFrame 字段跨帧/跨调用缓存.
//
// readStreamChunk reads more data for Conn.Read to consume.
//
// If this frame is just a control frame (ping/pong/close, already dispatched), it returns
// (nil, nil); the caller should call again to obtain actual data.
//
// Uncompressed messages are delivered frame by frame as soon as each frame is read, without
// waiting for subsequent fragments of the message. Compressed messages need the whole
// message's byte stream to be inflated together, so all fragments are accumulated and the
// whole message is decompressed once the final frame arrives. Whether the message is
// compressed, its opcode, etc. are cached across frames/calls by reusing c.continuationFrame.
func (c *Conn) readStreamChunk() (*bytes.Buffer, error) {
	// 解析帧头并获取内容长度
	// Parse the frame header and get the content length
	contentLength, err := c.fh.Parse(c.br)
	if err != nil {
		return nil, err
	}
	if contentLength > c.config.ReadMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}

	if !c.pd.Enabled && (c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3()) {
		return nil, internal.CloseProtocolError
	}

	maskEnabled := c.fh.GetMask()
	if err := c.checkMask(maskEnabled); err != nil {
		return nil, err
	}

	var opcode = c.fh.GetOpcode()
	if !opcode.isDataFrame() {
		return nil, c.readControl()
	}

	var fin = c.fh.GetFIN()
	var firstFrame = opcode != OpcodeContinuation

	if firstFrame {
		if c.continuationFrame.initialized {
			return nil, internal.CloseProtocolError
		}
		c.continuationFrame.initialized = true
		c.continuationFrame.opcode = opcode
		c.continuationFrame.compressed = c.pd.Enabled && c.fh.GetRSV1()
	} else if !c.continuationFrame.initialized {
		return nil, internal.CloseProtocolError
	}

	var buf = binaryPool.Get(contentLength + len(flateTail))
	var p = buf.Bytes()[:contentLength]

	// buf 默认在函数返回时回收, 除非其所有权被转移出去(此时由调用方负责回收)
	// buf is recycled on return by default, unless its ownership is transferred out
	// (in which case the caller is responsible for recycling it)
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

	if !c.continuationFrame.compressed {
		// 未分片/未压缩的消息只需要校验它自身; 跨帧拼装出来的消息不在这里做编码校验,
		// 因为一个 UTF8 字符可能被拆分到两个 frame 里
		// An unfragmented/uncompressed message only needs to validate itself; messages
		// reassembled across frames are not validated for encoding here, since a UTF8
		// character could be split across two frames
		if firstFrame && fin && !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(opcode), p) {
			return nil, internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
		}
		*(*[]byte)(unsafe.Pointer(buf)) = p
		recycle = false
		if fin {
			c.continuationFrame.reset()
		}
		return buf, nil
	}

	// 压缩消息: 累积分片, 读到最后一帧后整体解压
	// Compressed message: accumulate fragments, decompress as a whole once the final frame arrives
	if c.continuationFrame.buffer == nil {
		c.continuationFrame.buffer = bytes.NewBuffer(make([]byte, 0, contentLength))
	}
	c.continuationFrame.buffer.Write(p)
	if c.continuationFrame.buffer.Len() > c.config.ReadMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}
	if !fin {
		return nil, nil
	}

	msg := &Message{Opcode: c.continuationFrame.opcode, Data: c.continuationFrame.buffer, compressed: true}
	c.continuationFrame.reset()
	if err := c.processMessage(msg); err != nil {
		return nil, err
	}
	return msg.Data, nil
}

// 分发消息和异常恢复
// Dispatch message & Recovery
func (c *Conn) dispatchMessage(msg *Message) error {
	defer c.config.Recovery(c.config.Logger)
	c.handler.OnMessage(c, msg)
	return nil
}

// 分发控制帧事件并进行异常恢复
// Dispatch control-frame events with recovery
//
// 控制帧(Ping/Pong/Close)的回调如果发生 panic，不应直接导致 ReadLoop 崩溃；
// 因此这里统一通过 Config.Recovery 进行兜底。
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

// 处理消息: 解压并校验编码
// Processes a message: decompresses it and validates its encoding
func (c *Conn) processMessage(msg *Message) error {
	if msg.compressed {
		var rawBuf = msg.Data
		dst, err := c.deflater.Decompress(rawBuf, c.dpsWindow.dict)
		binaryPool.Put(rawBuf)
		if err != nil {
			msg.Data = nil
			return internal.NewError(internal.CloseInternalErr, err)
		}
		msg.Data = dst
		_, _ = c.dpsWindow.Write(msg.Bytes())
	}
	if !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(msg.Opcode), msg.Bytes()) {
		return internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}
	return nil
}

// 发射消息事件
// Emit onmessage event
func (c *Conn) emitMessage(msg *Message) error {
	if err := c.processMessage(msg); err != nil {
		return err
	}
	if c.config.ParallelEnabled {
		return c.readQueue.Go(msg, c.dispatchMessage)
	}
	return c.dispatchMessage(msg)
}
