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
// WriteMessage writes text/binary messages, text messages should be encoded in UTF8.
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	// 调用 doWrite 方法写入消息
	// Call the doWrite method to write the message
	err := c.doWrite(opcode, internal.Bytes(payload))

	// 触发错误处理
	// Emit error handling
	c.emitError(err)

	// 返回错误信息
	// Return the error
	return err
}

// WriteAsync 异步写
// WriteAsync writes messages asynchronously
// 异步非阻塞地将消息写入到任务队列, 收到回调后才允许回收payload内存
// Write messages to the task queue asynchronously and non-blockingly, allowing payload memory to be recycled only after receiving the callback
func (c *Conn) WriteAsync(opcode Opcode, payload []byte, callback func(error)) {
	// 将写操作推送到写队列中
	// Push the write operation to the write queue
	c.writeQueue.Push(func() {
		// 调用 WriteMessage 方法写入消息
		// Call the WriteMessage method to write the message
		if err := c.WriteMessage(opcode, payload); callback != nil {
			// 如果有回调函数，调用回调函数并传递错误信息
			// If there is a callback function, call it and pass the error
			callback(err)
		}
	})
}

// Writev 类似 WriteMessage, 区别是可以一次写入多个切片
// Writev is similar to WriteMessage, except that you can write multiple slices at once.
func (c *Conn) Writev(opcode Opcode, payloads ...[]byte) error {
	// 调用 doWrite 方法写入多个切片
	// Call the doWrite method to write multiple slices
	var err = c.doWrite(opcode, internal.Buffers(payloads))

	// 触发错误处理
	// Emit error handling
	c.emitError(err)

	// 返回错误信息
	// Return the error
	return err
}

// WritevAsync 类似 WriteAsync, 区别是可以一次写入多个切片
// WritevAsync is similar to WriteAsync, except that you can write multiple slices at once.
func (c *Conn) WritevAsync(opcode Opcode, payloads [][]byte, callback func(error)) {
	// 将写操作推送到写队列中
	// Push the write operation to the write queue
	c.writeQueue.Push(func() {
		// 调用 Writev 方法写入多个切片
		// Call the Writev method to write multiple slices
		if err := c.Writev(opcode, payloads...); callback != nil {
			// 如果有回调函数，调用回调函数并传递错误信息
			// If there is a callback function, call it and pass the error
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
// doWrite executes the write logic, ensuring proper maintenance of the compression dictionary
func (c *Conn) doWrite(opcode Opcode, payload internal.Payload) error {
	// 加锁以确保线程安全
	// Lock to ensure thread safety
	c.mu.Lock()
	// 在函数结束时解锁
	// Unlock at the end of the function
	defer c.mu.Unlock()

	// 如果操作码不是关闭连接且连接已关闭，返回连接关闭错误
	// If the opcode is not CloseConnection and the connection is closed, return a connection closed error
	if opcode != OpcodeCloseConnection && c.isClosed() {
		return ErrConnClosed
	}

	// 生成帧数据
	// Generate the frame data
	frame, err := c.genFrame(opcode, payload, false)
	if err != nil {
		return err
	}

	// 将帧数据写入连接
	// Write the frame data to the connection
	err = internal.WriteN(c.conn, frame.Bytes())

	// 将 payload 写入压缩窗口
	// Write the payload to the compression window
	_, _ = payload.WriteTo(&c.cpsWindow)

	// 将帧放回缓冲池
	// Put the frame back into the buffer pool
	binaryPool.Put(frame)

	// 返回写入操作的错误信息
	// Return the error from the write operation
	return err
}

// genFrame 生成帧数据
// genFrame generates the frame data
func (c *Conn) genFrame(opcode Opcode, payload internal.Payload, isBroadcast bool) (*bytes.Buffer, error) {
	// 如果操作码是文本且编码检查未通过，返回不支持的数据错误
	// If the opcode is text and the encoding check fails, return an unsupported data error
	if opcode == OpcodeText && !payload.CheckEncoding(c.config.CheckUtf8Enabled, uint8(opcode)) {
		return nil, internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}

	// 获取负载的长度
	// Get the length of the payload
	var n = payload.Len()

	// 如果负载长度超过配置的最大负载大小，返回消息过大错误
	// If the payload length exceeds the configured maximum payload size, return a message too large error
	if n > c.config.WriteMaxPayloadSize {
		return nil, internal.CloseMessageTooLarge
	}

	// 从缓冲池获取一个缓冲区，大小为负载长度加上帧头大小
	// Get a buffer from the buffer pool, with size equal to payload length plus frame header size
	var buf = binaryPool.Get(n + frameHeaderSize)

	// 写入帧填充数据
	// Write frame padding data
	buf.Write(framePadding[0:])

	// 如果启用了压缩且操作码是数据帧且负载长度大于等于压缩阈值，进行数据压缩
	// If compression is enabled, the opcode is a data frame, and the payload length is greater than or equal to the compression threshold, compress the data
	if c.pd.Enabled && opcode.isDataFrame() && n >= c.pd.Threshold {
		return c.compressData(buf, opcode, payload, isBroadcast)
	}

	// 生成帧头
	// Generate the frame header
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, false, opcode, n)

	// 将负载写入缓冲区
	// Write the payload to the buffer
	_, _ = payload.WriteTo(buf)

	// 获取缓冲区的字节切片
	// Get the byte slice of the buffer
	var contents = buf.Bytes()

	// 如果不是服务器端，进行掩码异或操作
	// If not server-side, perform mask XOR operation
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}

	// 计算帧头的偏移量
	// Calculate the offset of the frame header
	var m = frameHeaderSize - headerLength

	// 将帧头复制到缓冲区
	// Copy the frame header to the buffer
	copy(contents[m:], header[:headerLength])

	// 调整缓冲区的读取位置
	// Adjust the read position of the buffer
	buf.Next(m)

	// 返回缓冲区和 nil 错误
	// Return the buffer and nil error
	return buf, nil
}

// compressData 压缩数据并生成帧
// compressData compresses the data and generates the frame
func (c *Conn) compressData(buf *bytes.Buffer, opcode Opcode, payload internal.Payload, isBroadcast bool) (*bytes.Buffer, error) {
	// 使用 deflater 压缩数据并写入缓冲区
	// Use deflater to compress the data and write it to the buffer
	err := c.deflater.Compress(payload, buf, c.getCpsDict(isBroadcast))
	if err != nil {
		return nil, err
	}

	// 获取缓冲区的字节切片
	// Get the byte slice of the buffer
	var contents = buf.Bytes()

	// 计算压缩后的负载大小
	// Calculate the size of the compressed payload
	var payloadSize = buf.Len() - frameHeaderSize

	// 生成帧头
	// Generate the frame header
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, true, opcode, payloadSize)

	// 如果不是服务器端，进行掩码异或操作
	// If not server-side, perform mask XOR operation
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}

	// 计算帧头的偏移量
	// Calculate the offset of the frame header
	var m = frameHeaderSize - headerLength

	// 将帧头复制到缓冲区
	// Copy the frame header to the buffer
	copy(contents[m:], header[:headerLength])

	// 调整缓冲区的读取位置
	// Adjust the read position of the buffer
	buf.Next(m)

	// 返回缓冲区和 nil 错误
	// Return the buffer and nil error
	return buf, nil
}

type (
	// Broadcaster 结构体用于广播消息
	// Broadcaster struct is used for broadcasting messages
	Broadcaster struct {
		// opcode 表示操作码
		// opcode represents the operation code
		opcode Opcode

		// payload 表示消息的负载
		// payload represents the message payload
		payload []byte

		// msgs 是一个包含两个广播消息包装器的数组
		// msgs is an array containing two broadcast message wrappers
		msgs [2]*broadcastMessageWrapper

		// state 表示广播器的状态
		// state represents the state of the broadcaster
		state int64
	}

	// broadcastMessageWrapper 结构体用于包装广播消息
	// broadcastMessageWrapper struct is used to wrap broadcast messages
	broadcastMessageWrapper struct {
		// once 用于确保某些操作只执行一次
		// once is used to ensure certain operations are executed only once
		once sync.Once

		// err 表示广播消息的错误状态
		// err represents the error state of the broadcast message
		err error

		// frame 表示广播消息的帧数据
		// frame represents the frame data of the broadcast message
		frame *bytes.Buffer
	}
)

// NewBroadcaster 创建广播器
// NewBroadcaster creates a broadcaster
// 相比循环调用 WriteAsync, Broadcaster 只会压缩一次消息, 可以节省大量 CPU 开销.
// Instead of calling WriteAsync in a loop, Broadcaster compresses the message only once, saving a lot of CPU overhead.
func NewBroadcaster(opcode Opcode, payload []byte) *Broadcaster {
	// 初始化一个 Broadcaster 实例
	// Initialize a Broadcaster instance
	c := &Broadcaster{
		// 设置操作码
		// Set the operation code
		opcode: opcode,

		// 设置消息负载
		// Set the message payload
		payload: payload,

		// 初始化广播消息包装器数组
		// Initialize the broadcast message wrapper array
		msgs: [2]*broadcastMessageWrapper{{}, {}},

		// 设置初始状态
		// Set the initial state
		state: int64(math.MaxInt32),
	}

	// 返回 Broadcaster 实例
	// Return the Broadcaster instance
	return c
}

// writeFrame 将帧数据写入连接
// writeFrame writes the frame data to the connection
func (c *Broadcaster) writeFrame(socket *Conn, frame *bytes.Buffer) error {
	// 如果连接已关闭，返回连接关闭错误
	// If the connection is closed, return a connection closed error
	if socket.isClosed() {
		return ErrConnClosed
	}

	// 加锁以确保线程安全
	// Lock to ensure thread safety
	socket.mu.Lock()

	// 写入帧数据到连接
	// Write the frame data to the connection
	var err = internal.WriteN(socket.conn, frame.Bytes())

	// 将负载写入压缩窗口
	// Write the payload to the compression window
	socket.cpsWindow.Write(c.payload)

	// 解锁
	// Unlock
	socket.mu.Unlock()

	// 返回写入操作的错误信息
	// Return the error from the write operation
	return err
}

func (c *Broadcaster) Broadcast(socket *Conn) error {
	// 根据是否启用压缩选择索引值
	// Select index value based on whether compression is enabled
	var idx = internal.SelectValue(socket.pd.Enabled, 1, 0)
	// 获取对应索引的广播消息包装器
	// Get the broadcast message wrapper for the corresponding index
	var msg = c.msgs[idx]

	// 使用 sync.Once 确保帧数据只生成一次
	// Use sync.Once to ensure the frame data is generated only once
	msg.once.Do(func() {
		// 生成帧数据
		// Generate the frame data
		msg.frame, msg.err = socket.genFrame(c.opcode, internal.Bytes(c.payload), true)
	})

	// 如果生成帧数据时发生错误，返回错误
	// If there is an error generating the frame data, return the error
	if msg.err != nil {
		return msg.err
	}

	// 原子性地增加广播器的状态值
	// Atomically increment the state value of the broadcaster
	atomic.AddInt64(&c.state, 1)

	// 将写入操作推入连接的写队列
	// Push the write operation into the connection's write queue
	socket.writeQueue.Push(func() {
		// 将帧数据写入连接
		// Write the frame data to the connection
		var err = c.writeFrame(socket, msg.frame)

		// 触发错误事件
		// Emit the error event
		socket.emitError(err)

		// 原子性地减少广播器的状态值，如果状态值为 0，关闭广播器
		// Atomically decrement the state value of the broadcaster, if the state value is 0, close the broadcaster
		if atomic.AddInt64(&c.state, -1) == 0 {
			c.doClose()
		}
	})

	// 返回 nil 表示成功
	// Return nil to indicate success
	return nil
}

// doClose 关闭广播器并释放资源
// doClose closes the broadcaster and releases resources
func (c *Broadcaster) doClose() {
	// 遍历广播消息包装器数组
	// Iterate over the broadcast message wrapper array
	for _, item := range c.msgs {
		// 如果包装器不为空，释放其帧数据
		// If the wrapper is not nil, release its frame data
		if item != nil {
			binaryPool.Put(item.frame)
		}
	}
}

// Close 释放资源
// Close releases resources
// 在完成所有 Broadcast 调用之后执行 Close 方法释放资源。
// Call the Close method after all the Broadcasts have been completed to release the resources.
func (c *Broadcaster) Close() error {
	// 原子性地减少广播器的状态值
	// Atomically decrement the state value of the broadcaster
	if atomic.AddInt64(&c.state, -1*math.MaxInt32) == 0 {
		// 如果状态值为 0，关闭广播器并释放资源
		// If the state value is 0, close the broadcaster and release resources
		c.doClose()
	}

	// 返回 nil 表示成功
	// Return nil to indicate success
	return nil
}
