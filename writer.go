package gws

import (
	"bytes"
	"errors"
	"github.com/lxzan/gws/internal"
)

// WriteClose
// code: https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
// 通过emitError发送关闭帧, 将连接状态置为关闭, 用于服务端主动断开连接
// 没有特殊原因的话, 建议code=0, reason=nil
// Send a close frame via emitError to set the connection state to closed, for server-initiated disconnection
// If there is no special reason, we suggest code=0, reason=nil
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
func (c *Conn) WriteString(s string) error {
	return c.WriteMessage(OpcodeText, internal.StringToBytes(s))
}

// WriteAsync 异步非阻塞地写入消息
// Write messages asynchronously and non-blockingly
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) error {
	frame, index, err := c.genFrame(opcode, payload)
	if err != nil {
		c.emitError(err)
		return err
	}

	c.writeQueue.Push(func() {
		if c.isClosed() {
			return
		}
		err = internal.WriteN(c.conn, frame.Bytes(), frame.Len())
		myBufferPool.Put(frame, index)
		c.emitError(err)
	})
	return nil
}

// WriteMessage 发送消息
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
	frame, index, err := c.genFrame(opcode, payload)
	if err != nil {
		return err
	}

	err = internal.WriteN(c.conn, frame.Bytes(), frame.Len())
	myBufferPool.Put(frame, index)
	return err
}

// 帧生成
func (c *Conn) genFrame(opcode Opcode, payload []byte) (*bytes.Buffer, int, error) {
	// 不要删除 opcode == OpcodeText
	if opcode == OpcodeText && !c.isTextValid(opcode, payload) {
		return nil, 0, internal.NewError(internal.CloseUnsupportedData, internal.ErrTextEncoding)
	}

	if c.compressEnabled && opcode.isDataFrame() && len(payload) >= c.config.CompressThreshold {
		return c.compressData(opcode, payload)
	}

	var n = len(payload)
	if n > c.config.WriteMaxPayloadSize {
		return nil, 0, internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, false, opcode, n)
	var totalSize = n + headerLength
	var buf, index = myBufferPool.Get(totalSize)
	buf.Write(header[:headerLength])
	buf.Write(payload)
	var contents = buf.Bytes()
	if !c.isServer {
		internal.MaskXOR(contents[headerLength:], maskBytes)
	}
	return buf, index, nil
}

func (c *Conn) compressData(opcode Opcode, payload []byte) (*bytes.Buffer, int, error) {
	var buf, index = myBufferPool.Get(len(payload) / compressionRate)
	buf.Write(myPadding[0:])
	err := c.config.compressors.Select().Compress(payload, buf)
	if err != nil {
		return nil, 0, err
	}
	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	if payloadSize > c.config.WriteMaxPayloadSize {
		return nil, 0, internal.CloseMessageTooLarge
	}
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, true, opcode, payloadSize)
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	copy(contents[frameHeaderSize-headerLength:], header[:headerLength])
	buf.Next(frameHeaderSize - headerLength)
	return buf, index, nil
}
