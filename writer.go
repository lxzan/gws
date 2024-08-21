package gws

import (
	"bytes"
	"math"
	"sync"
	"sync/atomic"

	"github.com/lxzan/gws/internal"
)

// WriteClose 发送关闭帧并断开连接
// 没有特殊需求的话, 推荐code=1000, reason=nil
// Send shutdown frame, active disconnection
// If you don't have any special needs, we recommend code=1000, reason=nil
// https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
func (c *Conn) WriteClose(code uint16, reason []byte) error {
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		var buf = binaryPool.Get(128)
		code = internal.SelectValue(code < 1000, 1000, code)
		buf.Write(internal.StatusCode(code).Bytes())
		buf.Write(reason)
		err := c.writeClose(internal.StatusCode(code), buf.Bytes())
		binaryPool.Put(buf)
		return err
	}
	return ErrConnClosed
}

// 关闭连接并存储错误信息
// Closes the connection and stores the error information
func (c *Conn) writeClose(ev error, reason []byte) error {
	if len(reason) > internal.ThresholdV1 {
		reason = reason[:internal.ThresholdV1]
	}
	c.ev.Store(ev)
	err := c.doWrite(OpcodeCloseConnection, internal.Bytes(reason))
	_ = c.conn.Close()
	return err
}

// WritePing
// 写入Ping消息, 携带的信息不要超过125字节
// Control frame length cannot exceed 125 bytes
func (c *Conn) WritePing(payload []byte) error {
	return c.WriteMessage(OpcodePing, payload)
}

// WritePong
// 写入Pong消息, 携带的信息不要超过125字节
// Control frame length cannot exceed 125 bytes
func (c *Conn) WritePong(payload []byte) error {
	return c.WriteMessage(OpcodePong, payload)
}

// WriteString
// 写入文本消息, 使用UTF8编码.
// Write text messages, should be encoded in UTF8.
func (c *Conn) WriteString(s string) error {
	return c.WriteMessage(OpcodeText, internal.StringToBytes(s))
}

// WriteMessage
// 写入文本/二进制消息, 文本消息应该使用UTF8编码
// Writes text/binary messages, text messages should be encoded in UTF8.
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	err := c.doWrite(opcode, internal.Bytes(payload))
	c.emitError(false, err)
	return err
}

// WriteAsync 异步写
// Writes messages asynchronously
// 异步非阻塞地将消息写入到任务队列, 收到回调后才允许回收payload内存
// Write messages to the task queue asynchronously and non-blockingly,
// allowing payload memory to be recycled only after receiving the callback
func (c *Conn) WriteAsync(opcode Opcode, payload []byte, callback func(error)) {
	c.Async(func() {
		if err := c.WriteMessage(opcode, payload); callback != nil {
			callback(err)
		}
	})
}

// Writev
// 类似 WriteMessage, 区别是可以一次写入多个切片
// Writev is similar to WriteMessage, except that you can write multiple slices at once.
func (c *Conn) Writev(opcode Opcode, payloads ...[]byte) error {
	var err = c.doWrite(opcode, internal.Buffers(payloads))
	c.emitError(false, err)
	return err
}

// WritevAsync 类似 WriteAsync, 区别是可以一次写入多个切片
// It's similar to WriteAsync, except that you can write multiple slices at once.
func (c *Conn) WritevAsync(opcode Opcode, payloads [][]byte, callback func(error)) {
	c.Async(func() {
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
// Executes the write logic, ensuring proper maintenance of the compression dictionary
func (c *Conn) doWrite(opcode Opcode, payload internal.Payload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opcode != OpcodeCloseConnection && c.isClosed() {
		return ErrConnClosed
	}

	// 生成帧, 向连接写入内容, 最后更新压缩字典
	// 为了使上下文接管模式正常工作, 压缩, 写入和更新字典三个操作的上下文必须保持同步
	// Generate frames, write to the connection, and update the compression dictionary
	// For context_takeover mode to work correctly, the contexts of compression, writing, and dictionary updating must be synchronized.
	frame, err := c.genFrame(opcode, payload, frameConfig{
		fin:           true,
		compress:      c.pd.Enabled,
		broadcast:     false,
		checkEncoding: c.config.CheckUtf8Enabled,
	})
	if err != nil {
		return err
	}
	err = internal.WriteN(c.conn, frame.Bytes())
	_, _ = payload.WriteTo(&c.cpsWindow)
	binaryPool.Put(frame)
	return err
}

// WebSocket帧配置, 用于重写连接里面的配置, 以适配各种场景
// WebSocket frame configuration, used to rewrite the configuration inside the connection, to adapt to various scenarios
type frameConfig struct {
	// 结束标志位
	// Finish flag
	fin bool

	// 是否开启压缩
	// Whether to enable compression
	compress bool

	// 帧生成动作是否由广播发起
	// Whether the frame generation action is initiated by a broadcast
	broadcast bool

	// 是否检查文本编码
	// Whether to check text encoding
	checkEncoding bool
}

// 生成帧数据
// Generates the frame data
func (c *Conn) genFrame(opcode Opcode, payload internal.Payload, cfg frameConfig) (*bytes.Buffer, error) {
	var n = payload.Len()
	if opcode == OpcodeText && !payload.CheckEncoding(cfg.checkEncoding, uint8(opcode)) {
		return nil, ErrTextEncoding
	}
	if n > c.config.WriteMaxPayloadSize {
		return nil, ErrMessageTooLarge
	}

	var buf = binaryPool.Get(n + frameHeaderSize)
	buf.Write(framePadding[0:])

	if cfg.compress && opcode.isDataFrame() && n >= c.pd.Threshold {
		return c.compressData(opcode, payload, buf, cfg)
	}

	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, cfg.fin, false, opcode, n)
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

// 压缩数据并生成帧
// Compresses the data and generates the frame
func (c *Conn) compressData(opcode Opcode, payload internal.Payload, buf *bytes.Buffer, cfg frameConfig) (*bytes.Buffer, error) {
	// 广播模式必须保证每一帧都是相同的内容, 所以不能使用字典优化压缩率
	// Broadcast mode must ensure that every frame is the same, so you can't use a dictionary to optimize the compression rate.
	var dict = internal.SelectValue(cfg.broadcast, nil, c.cpsWindow.dict)
	if err := c.deflater.Compress(payload, buf, dict); err != nil {
		return nil, err
	}

	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, cfg.fin, true, opcode, payloadSize)
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
// Creates a broadcaster
// 相比循环调用 WriteAsync, Broadcaster 只会压缩一次消息, 可以节省大量 CPU 开销.
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

// 将帧数据写入连接
// Writes the frame data to the connection
func (c *Broadcaster) writeFrame(socket *Conn, frame *bytes.Buffer) error {
	if socket.isClosed() {
		return ErrConnClosed
	}
	socket.mu.Lock()
	var err = internal.WriteN(socket.conn, frame.Bytes())
	_, _ = socket.cpsWindow.Write(c.payload)
	socket.mu.Unlock()
	return err
}

// Broadcast 广播
// 向客户端发送广播消息
// Send a broadcast message to a client.
func (c *Broadcaster) Broadcast(socket *Conn) error {
	var idx = internal.SelectValue(socket.pd.Enabled, 1, 0)
	var msg = c.msgs[idx]

	msg.once.Do(func() {
		msg.frame, msg.err = socket.genFrame(c.opcode, internal.Bytes(c.payload), frameConfig{
			fin:           true,
			compress:      socket.pd.Enabled,
			broadcast:     true,
			checkEncoding: socket.config.CheckUtf8Enabled,
		})
	})
	if msg.err != nil {
		return msg.err
	}

	atomic.AddInt64(&c.state, 1)
	socket.writeQueue.Push(func() {
		var err = c.writeFrame(socket, msg.frame)
		socket.emitError(false, err)
		if atomic.AddInt64(&c.state, -1) == 0 {
			c.doClose()
		}
	})
	return nil
}

// 释放资源
// releases resources
func (c *Broadcaster) doClose() {
	for _, item := range c.msgs {
		if item != nil {
			binaryPool.Put(item.frame)
		}
	}
}

// Close 释放资源
// Releases resources
// 在完成所有 Broadcast 调用之后执行 Close 方法释放资源。
// Call the Close method after all the Broadcasts have been completed to release the resources.
func (c *Broadcaster) Close() error {
	if atomic.AddInt64(&c.state, -1*math.MaxInt32) == 0 {
		c.doClose()
	}
	return nil
}
