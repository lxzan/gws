package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"sync/atomic"
)

// WritePing write ping frame
func (c *Conn) WritePing(payload []byte) error {
	return c.Write(OpcodePing, payload)
}

// WritePong write pong frame
func (c *Conn) WritePong(payload []byte) error {
	return c.Write(OpcodePong, payload)
}

// WriteString write text frame
func (c *Conn) WriteString(s string) error {
	return c.Write(OpcodeText, internal.StringToBytes(s))
}

// Write writes message
// 发送消息
func (c *Conn) Write(opcode Opcode, payload []byte) error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return internal.ErrConnClosed
	}
	err := c.doWrite(opcode, payload)
	c.emitError(err)
	return err
}

// doWrite
// 关闭状态置为1后还能写, 以便发送关闭帧
func (c *Conn) doWrite(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	err := c.writePublic(opcode, payload)
	c.wmu.Unlock()
	return err
}

// 写入消息的公共逻辑
func (c *Conn) writePublic(opcode Opcode, payload []byte) error {
	var enableCompress = c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressionThreshold
	if enableCompress {
		compressedContent, err := c.compressor.Compress(bytes.NewBuffer(payload))
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = compressedContent.Bytes()
	}

	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(true, enableCompress, opcode, n)
	if err := internal.WriteN(c.wbuf, header[:headerLength], headerLength); err != nil {
		return err
	}
	if err := internal.WriteN(c.wbuf, payload, n); err != nil {
		return err
	}
	return c.wbuf.Flush()
}

// WriteAsync write message async
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) {
	c.wmq.Push(c, messageWrapper{
		opcode:  opcode,
		payload: payload,
	})
}

// write and clear messages
func doWriteAsync(conn *Conn) error {
	if conn.wmq.Len() == 0 {
		return nil
	}

	conn.wmu.Lock()
	err := conn.wmq.Range(func(msg messageWrapper) error {
		if atomic.LoadUint32(&conn.closed) == 1 {
			return internal.ErrConnClosed
		}
		return conn.writePublic(msg.opcode, msg.payload)
	})
	conn.wmu.Unlock()
	return err
}
