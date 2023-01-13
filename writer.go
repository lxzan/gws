package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"io"
	"strings"
	"sync/atomic"
)

func writeN(writer io.Writer, content []byte) error {
	var n = len(content)
	if n == 0 {
		return nil
	}
	num, err := writer.Write(content)
	if err != nil {
		return err
	}
	if num != n {
		return internal.NewError(internal.CloseInternalServerErr, internal.ErrUnexpectedWriting)
	}
	return nil
}

func copyN(writer io.Writer, reader internal.ReadLener) error {
	var n = int64(reader.Len())
	if n == 0 {
		return nil
	}
	num, err := io.CopyN(writer, reader, n)
	if err != nil {
		return err
	}
	if num != n {
		return internal.NewError(internal.CloseInternalServerErr, internal.ErrUnexpectedWriting)
	}
	return nil
}

// WritePing write ping frame
func (c *Conn) WritePing(payload []byte) {
	c.WriteMessage(OpcodePing, payload)
}

// WritePong write pong frame
func (c *Conn) WritePong(payload []byte) {
	c.WriteMessage(OpcodePong, payload)
}

// WriteBinary write binary frame
func (c *Conn) WriteBinary(payload []byte) {
	c.WriteMessage(OpcodeBinary, payload)
}

// WriteText write text frame
func (c *Conn) WriteText(payload string) {
	c.emitError(c.writeMessage(OpcodeText, strings.NewReader(payload)))
}

// WriteMessage write text/binary message
// text message must be utf8 encoding
// 发送文本/二进制消息, 文本消息必须是utf8编码
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) {
	c.emitError(c.writeMessage(opcode, bytes.NewBuffer(payload)))
}

func (c *Conn) writeMessage(opcode Opcode, payload internal.ReadLener) error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return nil
	}

	c.wmu.Lock()
	defer c.wmu.Unlock()

	var enableCompress = c.compressEnabled && opcode.IsDataFrame()
	if enableCompress {
		compressedContent, err := c.compressor.Compress(payload)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = bytes.NewBuffer(compressedContent)
	}
	return c.writeFrame(opcode, payload, enableCompress)
}

// 加锁是为了防止frame header和payload并发写入后乱序
// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload internal.ReadLener, enableCompress bool) error {
	var header = frameHeader{}
	var n = payload.Len()
	var headerLength = header.GenerateServerHeader(true, enableCompress, opcode, n)
	if err := writeN(c.wbuf, header[:headerLength]); err != nil {
		return err
	}
	if err := copyN(c.wbuf, payload); err != nil {
		return err
	}
	return c.wbuf.Flush()
}
