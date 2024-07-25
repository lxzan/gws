package gws

import (
	"bytes"
	"errors"
	"math"
	"sync"
	"sync/atomic"

	"github.com/lxzan/gws/internal"
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

// WriteMessage 写入文本/二进制消息, 文本消息应该使用UTF8编码
// Write text/binary messages, text messages should be encoded in UTF8.
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	err := c.doWrite(opcode, internal.Bytes(payload))
	c.emitError(err)
	return err
}

// WriteAsync 异步写
// 异步非阻塞地将消息写入到任务队列, 收到回调后才允许回收payload内存
// Asynchronously and non-blockingly write the message to the task queue, allowing the payload memory to be reclaimed only after a callback is received.
func (c *Conn) WriteAsync(opcode Opcode, payload []byte, callback func(error)) {
	c.writeQueue.Push(func() {
		if err := c.WriteMessage(opcode, payload); callback != nil {
			callback(err)
		}
	})
}

// Writev 类似WriteMessage, 区别是可以一次写入多个切片
// Similar to WriteMessage, except that you can write multiple slices at once.
func (c *Conn) Writev(opcode Opcode, payloads ...[]byte) error {
	var err = c.doWrite(opcode, internal.Buffers(payloads))
	c.emitError(err)
	return err
}

// WritevAsync 类似WriteAsync, 区别是可以一次写入多个切片
// Similar to WriteAsync, except that you can write multiple slices at once.
func (c *Conn) WritevAsync(opcode Opcode, payloads [][]byte, callback func(error)) {
	c.writeQueue.Push(func() {
		if err := c.Writev(opcode, payloads...); callback != nil {
			callback(err)
		}
	})
}

// Async 异步
// 将任务加入发送队列(并发度为1), 执行异步操作
// 注意: 不要加入长时间阻塞的任务
// Add the task to the send queue (concurrency 1), perform asynchronous operation.
// Note: Don't add tasks that are blocking for a long time.
func (c *Conn) Async(f func()) {
	c.writeQueue.Push(f)
}

// 执行写入逻辑, 注意妥善维护压缩字典
func (c *Conn) doWrite(opcode Opcode, payload internal.Payload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opcode != OpcodeCloseConnection && c.isClosed() {
		return ErrConnClosed
	}

	frame, err := c.genFrame(opcode, payload, false)
	if err != nil {
		return err
	}

	err = internal.WriteN(c.conn, frame.Bytes())
	_, _ = payload.WriteTo(&c.cpsWindow)
	binaryPool.Put(frame)
	return err
}

// 帧生成
func (c *Conn) genFrame(opcode Opcode, payload internal.Payload, isBroadcast bool) (*bytes.Buffer, error) {
	if opcode == OpcodeText && !payload.CheckEncoding(c.config.CheckUtf8Enabled, uint8(opcode)) {
		return nil, internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}

	var n = payload.Len()

	if n > c.config.WriteMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}

	var buf = binaryPool.Get(n*105/100 + frameHeaderSize)
	buf.Write(framePadding[0:])

	if c.pd.Enabled && opcode.isDataFrame() && n >= c.pd.Threshold {
		return c.compressData(buf, opcode, payload, isBroadcast)
	}

	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, false, opcode, n)
	_, _ = payload.WriteTo(buf)
	var contents = buf.Bytes()
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	var m = frameHeaderSize - headerLength
	copy(contents[m:], header[:headerLength])
	buf.Next(m)
	return buf, nil
}

func (c *Conn) compressData(buf *bytes.Buffer, opcode Opcode, payload internal.Payload, isBroadcast bool) (*bytes.Buffer, error) {
	err := c.deflater.Compress(payload, buf, c.getCpsDict(isBroadcast))
	if err != nil {
		return nil, err
	}
	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, true, opcode, payloadSize)
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	var m = frameHeaderSize - headerLength
	copy(contents[m:], header[:headerLength])
	buf.Next(m)
	return buf, nil
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

func (c *Broadcaster) writeFrame(socket *Conn, frame *bytes.Buffer) error {
	if socket.isClosed() {
		return ErrConnClosed
	}
	socket.mu.Lock()
	var err = internal.WriteN(socket.conn, frame.Bytes())
	socket.cpsWindow.Write(c.payload)
	socket.mu.Unlock()
	return err
}

// Broadcast 广播
// 向客户端发送广播消息
// Send a broadcast message to a client.
func (c *Broadcaster) Broadcast(socket *Conn) error {
	var idx = internal.SelectValue(socket.pd.Enabled, 1, 0)
	var msg = c.msgs[idx]

	msg.once.Do(func() { msg.frame, msg.err = socket.genFrame(c.opcode, internal.Bytes(c.payload), true) })
	if msg.err != nil {
		return msg.err
	}

	atomic.AddInt64(&c.state, 1)
	socket.writeQueue.Push(func() {
		var err = c.writeFrame(socket, msg.frame)
		socket.emitError(err)
		if atomic.AddInt64(&c.state, -1) == 0 {
			c.doClose()
		}
	})
	return nil
}

func (c *Broadcaster) doClose() {
	for _, item := range c.msgs {
		if item != nil {
			binaryPool.Put(item.frame)
			item.frame = nil
		}
	}
}

// Close 释放资源
// 在完成所有Broadcast调用之后执行Close方法释放资源.
// Call the Close method after all the Broadcasts have been completed to release the resources.
func (c *Broadcaster) Close() error {
	if atomic.AddInt64(&c.state, -1*math.MaxInt32) == 0 {
		c.doClose()
	}
	return nil
}
