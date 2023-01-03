package gws

import (
	"io"
	"math"
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

func (c *Conn) emitError(err error) {
	if err == nil {
		return
	}
	c.once.Do(func() {
		c.handlerError(err)
		c.handler.OnError(c, err)
	})
}

func (c *Conn) handlerError(err error) {
	code := CloseNormalClosure
	v, ok := err.(CloseCode)
	if ok {
		closeCode := v.Uint16()
		if closeCode < 1000 || (closeCode >= 1016 && closeCode < 3000) {
			code = CloseProtocolError
		} else {
			switch closeCode {
			case 1004, 1005, 1006, 1014:
				code = CloseProtocolError
			default:
				code = v
			}
		}
	}
	var content = code.Bytes()
	content = append(content, err.Error()...)
	if len(content) > math.MaxInt8 {
		content = content[:math.MaxInt8]
	}
	_ = c.writeMessage(OpcodeCloseConnection, content, true)
	c.handler.OnError(c, err)
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
	if len(content) > math.MaxInt8 {
		content = content[:math.MaxInt8]
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

// WriteMessage write message
// 发送消息
func (c *Conn) WriteMessage(messageType Opcode, content []byte) {
	c.emitError(c.writeMessage(messageType, content, true))
}

// WriteBatch write message in batch, call FlushWriter in the end
// 批量写入消息，最后一次写入后需要调用FlushWriter
func (c *Conn) WriteBatch(messageType Opcode, content []byte) {
	c.emitError(c.writeMessage(messageType, content, false))
}

// FlushWriter
// 刷新写入缓冲区
// flush write buffer
func (c *Conn) FlushWriter() {
	c.emitError(c.wbuf.Flush())
}

func (c *Conn) writeMessage(opcode Opcode, content []byte, flush bool) error {
	var enableCompress = c.compressEnabled && opcode.IsDataFrame()
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
