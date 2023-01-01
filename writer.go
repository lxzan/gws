package gws

import (
	"io"
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
	go func() { c.messageChan <- &Message{err: err} }()
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
	c.emitError(c.writeFrame(OpcodeCloseConnection, content, false))
}

// 发送消息
// send a message
func (c *Conn) Write(messageType Opcode, content []byte) {
	c.emitError(c.writeMessage(messageType, content))
}

func (c *Conn) writeMessage(opcode Opcode, content []byte) error {
	var enableCompress = c.compressEnabled && isDataFrame(opcode)
	if !enableCompress {
		return c.writeFrame(opcode, content, enableCompress)
	}

	compressedContent, err := c.compressor.Compress(content)
	if err != nil {
		return CloseInternalServerErr
	}
	return c.writeFrame(opcode, compressedContent, enableCompress)
}

// 加锁是为了防止frame header和payload并发写入后乱序
// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload []byte, enableCompress bool) error {
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
	if err := c.wbuf.Flush(); err != nil {
		return err
	}
	return c.netConn.SetWriteDeadline(time.Time{})
}
