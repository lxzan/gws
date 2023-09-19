package gws

import (
	"bytes"
	"errors"
	"github.com/lxzan/gws/internal"
	"math"
	"sync"
	"sync/atomic"
)

// WriteClose 发送关闭帧, 主动断开连接
// 没有特殊需求的话, 推荐code=1000, reason=nil
// Send shutdown frame, active disconnection
// If you don't have any special needs, we recommend code=1000, reason=nil
// https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
func (c *Conn) WriteClose(code uint16, reason []byte) {
	var err = internal.NewError(internal.StatusCode(code), errEmpty)
	if len(reason) > 0 {
		err.Err = errors.New(string(reason))
	}
	c.emitError(err)
}

// WritePing 写入Ping消息, 携带的信息不要超过125字节
// Control frame length cannot exceed 125 bytes
func (c *Conn) WritePing(payload []byte) error {
	return c.WriteMessage(OpcodePing, payload)
}

// WritePong 写入Pong消息, 携带的信息不要超过125字节
// Control frame length cannot exceed 125 bytes
func (c *Conn) WritePong(payload []byte) error {
	return c.WriteMessage(OpcodePong, payload)
}

// WriteString 写入文本消息, 使用UTF8编码.
// Write text messages, should be encoded in UTF8.
func (c *Conn) WriteString(s string) error {
	return c.WriteMessage(OpcodeText, internal.StringToBytes(s))
}

// WriteAsync 异步非阻塞地写入消息
// Write messages asynchronously and non-blocking
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) error {
	frame, index, err := c.genFrame(opcode, payload)
	if err != nil {
		c.emitError(err)
		return err
	}

	c.writeQueue.Push(func() {
		if c.isClosed() {
			return
		}
		err = internal.WriteN(c.conn, frame.Bytes())
		binaryPool.Put(frame, index)
		c.emitError(err)
	})
	return nil
}

// WriteMessage 写入文本/二进制消息, 文本消息应该使用UTF8编码
// Write text/binary messages, text messages should be encoded in UTF8.
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	if c.isClosed() {
		return ErrConnClosed
	}
	err := c.doWrite(opcode, payload)
	c.emitError(err)
	return err
}

// 执行写入逻辑, 关闭状态置为1后还能写, 以便发送关闭帧
// Execute the write logic, and write after the close state is set to 1, so that the close frame can be sent
func (c *Conn) doWrite(opcode Opcode, payload []byte) error {
	frame, index, err := c.genFrame(opcode, payload)
	if err != nil {
		return err
	}

	err = internal.WriteN(c.conn, frame.Bytes())
	binaryPool.Put(frame, index)
	return err
}

// 帧生成
func (c *Conn) genFrame(opcode Opcode, payload []byte) (*bytes.Buffer, int, error) {
	// 不要删除 opcode == OpcodeText
	if opcode == OpcodeText && !c.isTextValid(opcode, payload) {
		return nil, 0, internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}

	if c.compressEnabled && opcode.isDataFrame() && len(payload) >= c.config.CompressThreshold {
		return c.compressData(opcode, payload)
	}

	var n = len(payload)
	if n > c.config.WriteMaxPayloadSize {
		return nil, 0, internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, false, opcode, n)
	var buf, index = binaryPool.Get(n + headerLength)
	buf.Write(header[:headerLength])
	buf.Write(payload)
	var contents = buf.Bytes()
	if !c.isServer {
		internal.MaskXOR(contents[headerLength:], maskBytes)
	}
	return buf, index, nil
}

func (c *Conn) compressData(opcode Opcode, payload []byte) (*bytes.Buffer, int, error) {
	var buf, index = binaryPool.Get(len(payload) + frameHeaderSize)
	buf.Write(myPadding[0:])
	err := c.compressor.Compress(payload, buf)
	if err != nil {
		return nil, 0, err
	}
	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	if payloadSize > c.config.WriteMaxPayloadSize {
		return nil, 0, internal.CloseMessageTooLarge
	}
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, true, opcode, payloadSize)
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	copy(contents[frameHeaderSize-headerLength:], header[:headerLength])
	buf.Next(frameHeaderSize - headerLength)
	return buf, index, nil
}

type (
	Broadcaster struct {
		opcode  Opcode
		payload []byte
		msgs    [2]*broadcastMessageWrapper
		state   int64
	}

	broadcastMessageWrapper struct {
		once  sync.Once
		err   error
		index int
		frame *bytes.Buffer
	}
)

// NewBroadcaster 创建广播器
// 相比循环调用WriteAsync, Broadcaster只会压缩一次消息, 可以节省大量CPU开销.
// Instead of calling WriteAsync in a loop, Broadcaster compresses the message only once, saving a lot of CPU overhead.
func NewBroadcaster(opcode Opcode, payload []byte) *Broadcaster {
	c := &Broadcaster{
		opcode:  opcode,
		payload: payload,
		msgs:    [2]*broadcastMessageWrapper{{}, {}},
		state:   int64(math.MaxInt32),
	}
	return c
}

// Broadcast 广播
// 向客户端发送广播消息
// Send a broadcast message to a client.
func (c *Broadcaster) Broadcast(socket *Conn) error {
	var idx = internal.SelectValue(socket.compressEnabled, 1, 0)
	var msg = c.msgs[idx]
	msg.once.Do(func() { msg.frame, msg.index, msg.err = socket.genFrame(c.opcode, c.payload) })
	if msg.err != nil {
		return msg.err
	}

	atomic.AddInt64(&c.state, 1)
	socket.writeQueue.Push(func() {
		if !socket.isClosed() {
			socket.emitError(internal.WriteN(socket.conn, msg.frame.Bytes()))
		}
		if atomic.AddInt64(&c.state, -1) == 0 {
			c.doClose()
		}
	})
	return nil
}

func (c *Broadcaster) doClose() {
	for _, item := range c.msgs {
		if item != nil {
			binaryPool.Put(item.frame, item.index)
		}
	}
}

// Release 释放资源
// 在完成所有Broadcast调用之后执行Release方法释放资源.
// Call the Release method after all the Broadcasts have been completed to release the resources.
func (c *Broadcaster) Release() {
	if atomic.AddInt64(&c.state, -1*math.MaxInt32) == 0 {
		c.doClose()
	}
}
