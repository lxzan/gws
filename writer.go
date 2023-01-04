package gws

import (
	"github.com/lxzan/gws/internal"
	"io"
	"time"
)

func writeN(writer io.Writer, content []byte, n int) error {
	if n == 0 {
		return nil
	}
	num, err := writer.Write(content)
	if err != nil {
		return err
	}
	if num != n {
		return CloseGoingAway
	}
	return nil
}

// WriteClose write close frame
// 发送关闭帧
func (c *Conn) WriteClose(code CloseCode, reason []byte) {
	var content = code.Bytes()
	if len(content) > 0 {
		content = append(content, reason...)
	} else {
		content = append(content, code.Error()...)
	}
	if len(content) > internal.Lv1 {
		content = content[:internal.Lv1]
	}
	c.WriteMessage(OpcodeCloseConnection, content)
}

// WritePing write ping frame
func (c *Conn) WritePing(payload []byte) {
	c.WriteMessage(OpcodePing, payload)
}

// WritePong write pong frame
func (c *Conn) WritePong(payload []byte) {
	c.WriteMessage(OpcodePong, payload)
}

// WriteMessage write text/binary message
// text message must be utf8 encoding
// 发送文本/二进制消息, 文本消息必须是utf8编码
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) {
	c.emitError(c.writeMessage(opcode, payload, true))
}

// WriteBatch write message in batch, call FlushWriter in the end
// 批量写入消息，最后一次写入后需要调用FlushWriter
func (c *Conn) WriteBatch(opcode Opcode, payload []byte) {
	c.emitError(c.writeMessage(opcode, payload, false))
}

// FlushWriter
// 刷新写入缓冲区
// flush write buffer
func (c *Conn) FlushWriter() {
	c.emitError(c.wbuf.Flush())
}

func (c *Conn) writeMessage(opcode Opcode, payload []byte, flush bool) error {
	var enableCompress = c.compressEnabled && opcode.IsDataFrame()
	if enableCompress {
		compressedContent, err := c.compressor.Compress(payload)
		if err != nil {
			return CloseInternalServerErr
		}
		payload = compressedContent
	}
	return c.writeFrame(opcode, payload, enableCompress, flush)
}

// 加锁是为了防止frame header和payload并发写入后乱序
// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload []byte, enableCompress bool, flush bool) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(opcode, enableCompress, n)
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.configs.WriteTimeout)); err != nil {
		return err
	}

	if err := writeN(c.wbuf, header[:headerLength], headerLength); err != nil {
		return err
	}
	if err := writeN(c.wbuf, payload, n); err != nil {
		return err
	}
	if flush {
		if err := c.wbuf.Flush(); err != nil {
			return err
		}
	}
	return c.conn.SetWriteDeadline(time.Time{})
}
