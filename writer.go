package gws

import (
	"time"
)

// send ping frame
func (c *Conn) WritePing(payload []byte) {
	if err := c.writeFrame(OpcodePing, payload, false); err != nil {
		c.emitError(err)
		return
	}
}

// send pong frame
func (c *Conn) WritePong(payload []byte) {
	if err := c.writeFrame(OpcodePong, payload, false); err != nil {
		c.emitError(err)
		return
	}
}

// 发送消息
// send a message
func (c *Conn) Write(messageType Opcode, content []byte) {
	if err := c.prepareMessage(messageType, content); err != nil {
		c.emitError(err)
		return
	}
}

func (c *Conn) prepareMessage(opcode Opcode, content []byte) error {
	var enableCompress = c.compressEnabled && isDataFrame(opcode)
	if !enableCompress {
		return c.writeFrame(opcode, content, enableCompress)
	}

	var compressor = c.compressors.Select()
	compressedContent, err := compressor.Compress(content)
	defer compressor.Close()
	if err != nil {
		c.debugLog(err)
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
	defer c.wmu.Unlock()

	if err := c.netConn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout)); err != nil {
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
