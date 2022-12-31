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

// 发送消息
// send a message
func (c *Conn) Write(messageType Opcode, content []byte) {
	if err := c.prepareMessage(messageType, content); err != nil {
		go func() { c.messageChan <- &Message{err: err} }()
		return
	}
}

func (c *Conn) prepareMessage(opcode Opcode, content []byte) error {
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
	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(opcode, enableCompress, n)

	c.wmu.Lock()
	defer func() {
		c.wmu.Unlock()
		_ = c.netConn.SetWriteDeadline(time.Time{})
	}()

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
	return nil
}
