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

// WriteMessage writes message
// 发送消息
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return internal.ErrConnClosed
	}
	err := c.doWrite(opcode, payload)
	c.emitError(err)
	return err
}

// doWrite
// 关闭状态置为1后还能写, 以便发送关闭帧
func (c *Conn) doWrite(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	err := c.writePublic(opcode, payload)
	c.wmu.Unlock()
	return err
}

// 写入消息的公共逻辑
func (c *Conn) writePublic(opcode Opcode, payload []byte) error {
	var enableCompress = c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressionThreshold
	if enableCompress {
		compressedContent, err := c.compressor.Compress(bytes.NewBuffer(payload))
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = compressedContent.Bytes()
	}

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

// WriteAsync
// 异步写入消息, 适合广播等需要非阻塞的场景
// asynchronous write messages, suitable for non-blocking scenarios such as broadcasting
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) {
	// 不允许加任务了
	if atomic.LoadUint32(&c.closed) == 1 {
		return
	}

	c.wmq.Push(messageWrapper{opcode: opcode, payload: payload})
	c.aiomq.AddJob(asyncJob{Do: c.doWriteAsync})
}

func (c *Conn) doWriteAsync(args interface{}) error {
	if c.wmq.Len() == 0 {
		return nil
	}

	c.wmq.Lock()
	msgs := c.wmq.data
	c.wmq.data = []messageWrapper{}
	c.wmq.Unlock()

	myerr := func() error {
		c.wmu.Lock()
		defer c.wmu.Unlock()

		for _, msg := range msgs {
			if err := c.writePublic(msg.opcode, msg.payload); err != nil {
				return err
			}
		}
		return nil
	}()
	c.emitError(myerr)
	return myerr
}
