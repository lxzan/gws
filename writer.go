package gws

import (
	"bytes"
	"errors"
	"github.com/lxzan/gws/internal"
	"sync/atomic"
)

// WriteClose proactively close the connection
// code: https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
// 通过emitError发送关闭帧, 将连接状态置为关闭, 用于服务端主动断开连接
// 没有特殊原因的话, 建议code=0, reason=nil
func (c *Conn) WriteClose(code uint16, reason []byte) {
	var err = internal.NewError(internal.StatusCode(code), internal.GwsError(""))
	if len(reason) > 0 {
		err.Err = errors.New(string(reason))
	}
	c.emitError(err)
}

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

// 关闭状态置为1后还能写, 以便发送关闭帧
func (c *Conn) doWrite(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	err := c.writePublic(opcode, payload)
	c.wmu.Unlock()
	return err
}

// 写入消息的公共逻辑
func (c *Conn) writePublic(opcode Opcode, payload []byte) error {
	var useCompress = c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressionThreshold
	if useCompress {
		compressedContent, err := c.compressor.Compress(bytes.NewBuffer(payload))
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = compressedContent.Bytes()
	}
	if len(payload) > c.config.MaxContentLength {
		return internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(true, useCompress, opcode, n)
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
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) error {
	// 不允许加任务了
	if atomic.LoadUint32(&c.closed) == 1 {
		return internal.ErrConnClosed
	}
	//if err := c.writeMQ.Push(opcode, payload); err != nil {
	//	return err
	//}
	//c.writeTQ.Go(c.doWriteAsync)
	c.writeTQ.Push(func() { c.emitError(c.writePublic(opcode, payload)) })
	return nil
}

// 如果只有任务队列一个结构, 异步写有可能会因为和同步写竞争锁, 造成阻塞
// 所以我增加了一个消息队列
func (c *Conn) doWriteAsync() {
	myerr := func() error {
		msgs := c.writeMQ.PopAll()
		if len(msgs) == 0 {
			return nil
		}

		c.wmu.Lock()
		for _, msg := range msgs {
			if err := c.writePublic(msg.opcode, msg.payload); err != nil {
				c.wmu.Unlock()
				return err
			}
		}
		c.wmu.Unlock()
		return nil
	}()
	c.emitError(myerr)
}
