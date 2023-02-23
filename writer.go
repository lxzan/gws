package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"sync/atomic"
)

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

// WriteMessage write text/binary message
// 发送文本/二进制消息
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return internal.ErrConnClosed
	}
	err := c.doWriteMessage(opcode, payload)
	c.emitError(err)
	return err
}

// doWriteMessage
// 关闭状态置为1后还能写, 以便发送关闭帧
func (c *Conn) doWriteMessage(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var enableCompress = c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressionThreshold
	if enableCompress {
		compressedContent, err := c.compressor.Compress(bytes.NewBuffer(payload))
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = compressedContent.Bytes()
	}
	return c.writeFrame(opcode, payload, enableCompress)
}

// 加锁是为了防止frame header和payload并发写入后乱序
// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload []byte, enableCompress bool) error {
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

// WriteMessageAsync write message async
func (c *Conn) WriteMessageAsync(opcode Opcode, payload []byte) {
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
	defer conn.wmu.Unlock()
	return conn.wmq.Range(func(msg messageWrapper) error {
		if atomic.LoadUint32(&conn.closed) == 1 {
			return internal.ErrConnClosed
		}

		var enableCompress = conn.compressEnabled && msg.opcode.IsDataFrame() && len(msg.payload) >= conn.config.CompressionThreshold
		if enableCompress {
			compressedContent, err := conn.compressor.Compress(bytes.NewBuffer(msg.payload))
			if err != nil {
				return internal.NewError(internal.CloseInternalServerErr, err)
			}
			msg.payload = compressedContent.Bytes()
		}

		var header = frameHeader{}
		var n = len(msg.payload)
		var headerLength = header.GenerateServerHeader(true, enableCompress, msg.opcode, n)
		if err := internal.WriteN(conn.wbuf, header[:headerLength], headerLength); err != nil {
			return err
		}
		if err := internal.WriteN(conn.wbuf, msg.payload, n); err != nil {
			return err
		}
		return conn.wbuf.Flush()
	})
}
