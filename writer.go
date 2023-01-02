package gws

import (
	"io"
	"math"
	"time"
)

func writeN(writer io.Writer, content []byte, n int) error {
	num, err := writer.Write(content)
	if err != nil {
		return err
	}
	if num != n {
		return CloseGoingAway
	}
	return nil
}

func (c *Conn) emitError(err error) {
	if err == nil {
		return
	}
	go func() {
		code := CloseNormalClosure
		if v, ok := err.(CloseCode); ok && v.Uint16() > 0 {
			code = v
		}
		var content = code.Bytes()
		content = append(content, err.Error()...)
		if len(content) > math.MaxInt8 {
			content = content[:math.MaxInt8]
		}
		_ = c.writeFrame(OpcodeCloseConnection, content, false, true)
		c.messageChan <- &Message{err: err}
	}()
}

// WriteClose send close frame
// 发送关闭帧
func (c *Conn) WriteClose(code CloseCode, reason []byte) {
	var content = code.Bytes()
	if len(content) > 0 {
		content = append(content, reason...)
	} else {
		content = append(content, code.Error()...)
	}
	c.emitError(c.writeFrame(OpcodeCloseConnection, content, false, true))
}

// WriteMessage  send message
// 发送消息
func (c *Conn) WriteMessage(messageType Opcode, content []byte) {
	c.emitError(c.writeMessage(messageType, content, true))
}

// WriteBatch
// 批量写入消息，最后一次写入后需要调用Flush
func (c *Conn) WriteBatch(messageType Opcode, content []byte) {
	c.emitError(c.writeMessage(messageType, content, false))
}

// Flush
// 刷新写入缓冲区
// flush write buffer
func (c *Conn) Flush() {
	c.emitError(c.wbuf.Flush())
}

func (c *Conn) writeMessage(opcode Opcode, content []byte, flush bool) error {
	var enableCompress = c.compressEnabled && isDataFrame(opcode)
	if !enableCompress {
		return c.writeFrame(opcode, content, enableCompress, flush)
	}

	compressedContent, err := c.compressor.Compress(content)
	if err != nil {
		return CloseInternalServerErr
	}
	return c.writeFrame(opcode, compressedContent, enableCompress, flush)
}

// 加锁是为了防止frame header和payload并发写入后乱序
// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload []byte, enableCompress bool, flush bool) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(opcode, enableCompress, n)
	if err := c.netConn.SetWriteDeadline(time.Now().Add(c.configs.WriteTimeout)); err != nil {
		return err
	}

	if err := writeN(c.wbuf, header[:headerLength], headerLength); err != nil {
		return err
	}
	if n > 0 {
		if err := writeN(c.wbuf, payload, n); err != nil {
			return err
		}
	}
	if flush {
		if err := c.wbuf.Flush(); err != nil {
			return err
		}
	}
	return c.netConn.SetWriteDeadline(time.Time{})
}
