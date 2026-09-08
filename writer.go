package gws

import (
	"bytes"
	"errors"
	"math"
	"sync"
	"sync/atomic"

	"github.com/lxzan/gws/internal"
)

var (
	// ErrControlFrameTooLarge 控制帧载荷超过125字节
	ErrControlFrameTooLarge = errors.New("gws: control frame payload too large")

	// ErrReservedCloseCode 关闭帧使用了保留状态码(1005/1006/1015)
	ErrReservedCloseCode = errors.New("gws: reserved close code")
)

// WriteClose 发送关闭帧并断开连接.
// 没有特殊需求的话, 推荐code=1000, reason=nil.
// https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
func (c *Conn) WriteClose(code uint16, reason []byte) error {
	switch code {
	case 1005, 1006, 1015:
		return ErrReservedCloseCode
	}
	if len(reason)+2 > internal.ThresholdV1 {
		return ErrControlFrameTooLarge
	}
	if !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(OpcodeCloseConnection), reason) {
		return ErrTextEncoding
	}
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

// writeClose 关闭连接并存储错误信息
func (c *Conn) writeClose(ev error, reason []byte) error {
	if len(reason) > internal.ThresholdV1 {
		reason = reason[:internal.ThresholdV1]
	}
	c.ev.Store(ev)
	err := c.doWriteBytes(OpcodeCloseConnection, reason)
	_ = c.conn.Close()
	return err
}

// WritePing 写入Ping消息, 携带的信息不要超过125字节
func (c *Conn) WritePing(payload []byte) error {
	return c.WriteMessage(OpcodePing, payload)
}

// WritePong 写入Pong消息, 携带的信息不要超过125字节
func (c *Conn) WritePong(payload []byte) error {
	return c.WriteMessage(OpcodePong, payload)
}

// WriteString 写入文本消息, 使用UTF8编码
func (c *Conn) WriteString(s string) error {
	return c.WriteMessage(OpcodeText, internal.StringToBytes(s))
}

// WriteMessage 写入文本/二进制消息, 文本消息应该使用UTF8编码
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	err := c.doWriteBytes(opcode, payload)
	c.emitError(false, err)
	return err
}

// WriteAsync 异步写入消息.
// 异步非阻塞地将消息写入到任务队列, 收到回调后才允许回收payload内存.
func (c *Conn) WriteAsync(opcode Opcode, payload []byte, callback func(error)) {
	c.Async(func() {
		if err := c.WriteMessage(opcode, payload); callback != nil {
			callback(err)
		}
	})
}

// Writev 类似 WriteMessage, 区别是可以一次写入多个切片
func (c *Conn) Writev(opcode Opcode, payloads ...[]byte) error {
	var err = c.doWrite(opcode, internal.Buffers(payloads))
	c.emitError(false, err)
	return err
}

// WritevAsync 类似 WriteAsync, 区别是可以一次写入多个切片
func (c *Conn) WritevAsync(opcode Opcode, payloads [][]byte, callback func(error)) {
	c.Async(func() {
		if err := c.Writev(opcode, payloads...); callback != nil {
			callback(err)
		}
	})
}

// Async 将任务加入发送队列(并发度为1), 执行异步操作.
// 注意: 不要加入长时间阻塞的任务.
func (c *Conn) Async(f func()) {
	c.writeQueue.Push(f)
}

// doWrite 执行写入逻辑, 注意妥善维护压缩字典
func (c *Conn) doWrite(opcode Opcode, payload internal.Payload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opcode != OpcodeCloseConnection && c.isClosed() {
		return ErrConnClosed
	}

	// 生成帧, 向连接写入内容, 最后更新压缩字典.
	// 为了使上下文接管模式正常工作, 压缩, 写入和更新字典三个操作的上下文必须保持同步.
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
	if err == nil {
		_, _ = payload.WriteTo(&c.cpsWindow)
	}
	binaryPool.Put(frame)
	return err
}

func (c *Conn) doWriteBytes(opcode Opcode, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opcode != OpcodeCloseConnection && c.isClosed() {
		return ErrConnClosed
	}

	frame, err := c.genFrameBytes(opcode, payload, frameConfig{
		fin:           true,
		compress:      c.pd.Enabled,
		broadcast:     false,
		checkEncoding: c.config.CheckUtf8Enabled,
	})
	if err != nil {
		return err
	}
	err = internal.WriteN(c.conn, frame.Bytes())
	if err == nil {
		_, _ = c.cpsWindow.Write(payload)
	}
	binaryPool.Put(frame)
	return err
}

// frameConfig WebSocket帧配置, 用于重写连接里面的配置, 以适配各种场景
type frameConfig struct {
	fin           bool // 结束标志位
	compress      bool // 是否开启压缩
	broadcast     bool // 帧生成动作是否由广播发起
	checkEncoding bool // 是否检查文本编码
}

// genFrame 生成帧数据
func (c *Conn) genFrame(opcode Opcode, payload internal.Payload, cfg frameConfig) (*bytes.Buffer, error) {
	var n = payload.Len()
	if opcode == OpcodeText && !payload.CheckEncoding(cfg.checkEncoding, uint8(opcode)) {
		return nil, ErrTextEncoding
	}
	if !opcode.isDataFrame() && n > internal.ThresholdV1 {
		return nil, ErrControlFrameTooLarge
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
	headerLength, maskBytes := header.GenerateHeader(c.isServer, cfg.fin, false, opcode, n, c.nextMaskKey())
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

func (c *Conn) genFrameBytes(opcode Opcode, payload []byte, cfg frameConfig) (*bytes.Buffer, error) {
	var n = len(payload)
	if opcode == OpcodeText && !internal.CheckEncoding(cfg.checkEncoding, uint8(opcode), payload) {
		return nil, ErrTextEncoding
	}
	if !opcode.isDataFrame() && n > internal.ThresholdV1 {
		return nil, ErrControlFrameTooLarge
	}
	if n > c.config.WriteMaxPayloadSize {
		return nil, ErrMessageTooLarge
	}

	var buf = binaryPool.Get(n + frameHeaderSize)
	buf.Write(framePadding[0:])

	if cfg.compress && opcode.isDataFrame() && n >= c.pd.Threshold {
		return c.compressDataBytes(opcode, payload, buf, cfg)
	}

	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, cfg.fin, false, opcode, n, c.nextMaskKey())
	_, _ = buf.Write(payload)
	var contents = buf.Bytes()
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	var m = frameHeaderSize - headerLength
	copy(contents[m:], header[:headerLength])
	buf.Next(m)
	return buf, nil
}

// compressData 压缩数据并生成帧
func (c *Conn) compressData(opcode Opcode, payload internal.Payload, buf *bytes.Buffer, cfg frameConfig) (*bytes.Buffer, error) {
	// 广播模式必须保证每一帧都是相同的内容, 所以不能使用字典优化压缩率
	var dict = internal.SelectValue(cfg.broadcast, nil, c.cpsWindow.dict)
	if err := c.deflater.Compress(payload, buf, dict); err != nil {
		return nil, err
	}

	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, cfg.fin, true, opcode, payloadSize, c.nextMaskKey())
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	var m = frameHeaderSize - headerLength
	copy(contents[m:], header[:headerLength])
	buf.Next(m)
	return buf, nil
}

func (c *Conn) compressDataBytes(opcode Opcode, payload []byte, buf *bytes.Buffer, cfg frameConfig) (*bytes.Buffer, error) {
	// 广播模式必须保证每一帧都是相同的内容, 所以不能使用字典优化压缩率
	var dict = internal.SelectValue(cfg.broadcast, nil, c.cpsWindow.dict)
	if err := c.deflater.CompressBytes(payload, buf, dict); err != nil {
		return nil, err
	}

	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, cfg.fin, true, opcode, payloadSize, c.nextMaskKey())
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

// NewBroadcaster 创建广播器.
// 相比循环调用 WriteAsync, Broadcaster 只会压缩一次消息, 可以节省大量 CPU 开销.
func NewBroadcaster(opcode Opcode, payload []byte) *Broadcaster {
	c := &Broadcaster{
		opcode:  opcode,
		payload: payload,
		msgs:    [2]*broadcastMessageWrapper{{}, {}},
		state:   int64(math.MaxInt32),
	}
	return c
}

// writeFrame 将帧数据写入连接
func (c *Broadcaster) writeFrame(socket *Conn, frame *bytes.Buffer) error {
	if socket.isClosed() {
		return ErrConnClosed
	}
	socket.mu.Lock()
	var err = internal.WriteN(socket.conn, frame.Bytes())
	if err == nil {
		_, _ = socket.cpsWindow.Write(c.payload)
	}
	socket.mu.Unlock()
	return err
}

// Broadcast 向客户端发送广播消息, 并在写入完成后执行回调.
// 异步非阻塞地将消息写入到任务队列, 收到回调后才允许回收 payload 内存.
func (c *Broadcaster) Broadcast(socket *Conn, callback func(error)) error {
	var idx = internal.SelectValue(socket.pd.Enabled, 1, 0)
	var msg = c.msgs[idx]

	msg.once.Do(func() {
		msg.frame, msg.err = socket.genFrameBytes(c.opcode, c.payload, frameConfig{
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
		if callback != nil {
			callback(err)
		}
		if atomic.AddInt64(&c.state, -1) == 0 {
			c.doClose()
		}
	})
	return nil
}

// doClose 释放资源
func (c *Broadcaster) doClose() {
	for _, item := range c.msgs {
		if item != nil {
			binaryPool.Put(item.frame)
		}
	}
}

// Close 释放资源.
// 在完成所有 Broadcast 调用之后执行 Close 方法释放资源.
func (c *Broadcaster) Close() error {
	if atomic.AddInt64(&c.state, -1*math.MaxInt32) == 0 {
		c.doClose()
	}
	return nil
}
