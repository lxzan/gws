package gws

import (
	"errors"
	"github.com/lxzan/gws/internal"
)

// WriteClose proactively close the connection
// code: https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
// 通过emitError发送关闭帧, 将连接状态置为关闭, 用于服务端主动断开连接
// 没有特殊原因的话, 建议code=0, reason=nil
func (c *Conn) WriteClose(code uint16, reason []byte) {
	var err = internal.NewError(internal.StatusCode(code), internal.GwsError(""))
	if len(reason) > 0 {
		err.Err = errors.New(string(reason))
	}
	c.emitError(err)
}

// WritePing write ping frame
func (c *Conn) WritePing(payload []byte) error {
	return c.WriteMessage(OpcodePing, payload)
}

// WritePong write pong frame
func (c *Conn) WritePong(payload []byte) error {
	return c.WriteMessage(OpcodePong, payload)
}

// WriteString write text frame
// force convert string to []byte
func (c *Conn) WriteString(s string) error {
	return c.WriteMessage(OpcodeText, []byte(s))
}

// WriteMessage 发送消息
// 如果是客户端, payload内容会因为异或计算而被改变
// If it is a client, the payload content will be changed due to heterogeneous computation
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	if c.isClosed() {
		return internal.ErrConnClosed
	}
	err := c.doWrite(opcode, payload)
	c.emitError(err)
	return err
}

// 执行写入逻辑, 关闭状态置为1后还能写, 以便发送关闭帧
// Execute the write logic, and write after the close state is set to 1, so that the close frame can be sent
func (c *Conn) doWrite(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	if opcode == OpcodeText && !c.isTextValid(opcode, payload) {
		return internal.NewError(internal.CloseUnsupportedData, internal.ErrTextEncoding)
	}

	if c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressThreshold {
		return c.writeCompressedContents(opcode, payload)
	}

	var n = len(payload)
	if n > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, false, opcode, n)
	if !c.isServer {
		internal.MaskXOR(payload, maskBytes)
	}
	var totalSize = n + headerLength
	var buf = _bpool.Get(totalSize)
	buf.Write(header[:headerLength])
	buf.Write(payload)
	var err = internal.WriteN(c.conn, buf.Bytes(), totalSize)
	_bpool.Put(buf)
	return err
}

func (c *Conn) writeCompressedContents(opcode Opcode, payload []byte) error {
	var buf = _bpool.Get(len(payload) / 3)
	defer _bpool.Put(buf)

	var header = frameHeader{}
	buf.Write(header[0:])
	if err := c.compressor.Compress(payload, buf); err != nil {
		return err
	}

	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	if payloadSize > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, true, opcode, payloadSize)
	var offset = frameHeaderSize - headerLength
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	copy(contents[offset:frameHeaderSize], header[:headerLength])
	return internal.WriteN(c.conn, contents[offset:], payloadSize+headerLength)
}

// WriteAsync 异步非阻塞地写入消息
// Write messages asynchronously and non-blockingly
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) error {
	if c.isClosed() {
		return internal.ErrConnClosed
	}
	return c.writeQueue.Push(func() { c.emitError(c.doWrite(opcode, payload)) })
}
