package gws

import (
	"time"
)

// send ping frame
func (c *Conn) WritePing(payload []byte) {
	c.emitError(c.writeFrame(OpcodePing, payload, false))
}

// send pong frame
func (c *Conn) WritePong(payload []byte) {
	c.emitError(c.writeFrame(OpcodePong, payload, false))
}

// send close frame
func (c *Conn) WriteClose(code Code, reason []byte) {
	var content = code.Bytes()
	content = append(content, reason...)
	c.emitError(c.writeFrame(OpcodeCloseConnection, content, false))
}

// 发送消息; 此方法会回收内存, 不要用来写控制帧
// send a message; this method reclaims memory and should not be used to write control frames
func (c *Conn) Write(messageType Opcode, content []byte) {
	c.emitError(c.writeMessage(messageType, content))
}

func (c *Conn) writeMessage(opcode Opcode, content []byte) error {
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
	c.mu.Lock()
	defer func() {
		_ = c.netConn.SetWriteDeadline(time.Time{})
		c.mu.Unlock()
	}()

	if err := c.netConn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout)); err != nil {
		return err
	}
	if err := writeN(c.netConn, header[:headerLength], headerLength); err != nil {
		return err
	}
	if n > 0 {
		if err := writeN(c.netConn, payload, n); err != nil {
			return err
		}
	}
	return nil
}
