package gws

import (
	"github.com/lxzan/gws/internal"
	"io"
	"sync/atomic"
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
		return internal.CloseGoingAway
	}
	return nil
}

// WriteClose write close frame
// 发送关闭帧
func (c *Conn) WriteClose(code uint16, reason []byte) {
	var statusCode = internal.StatusCode(code)
	var content = statusCode.Bytes()
	if len(reason) > 0 {
		content = append(content, reason...)
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
	if atomic.LoadUint32(&c.closed) == 1 {
		return
	}
	c.emitError(c.writeMessage(opcode, payload))
}

func (c *Conn) writeMessage(opcode Opcode, payload []byte) error {
	var enableCompress = c.compressEnabled && opcode.IsDataFrame()
	if enableCompress {
		compressedContent, err := c.compressor.Compress(payload)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = compressedContent
	}
	return c.writeFrame(opcode, payload, enableCompress)
}

// 加锁是为了防止frame header和payload并发写入后乱序
// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload []byte, enableCompress bool) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(true, enableCompress, opcode, n)
	if err := writeN(c.wbuf, header[:headerLength], headerLength); err != nil {
		return err
	}
	if err := writeN(c.wbuf, payload, n); err != nil {
		return err
	}
	return c.wbuf.Flush()
}
