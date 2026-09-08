package gws

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"runtime"
	"unsafe"

	"github.com/lxzan/gws/internal"
)

const frameHeaderSize = 14

// Opcode 操作码
type Opcode uint8

const (
	OpcodeContinuation    Opcode = 0x0 // 	继续
	OpcodeText            Opcode = 0x1 // 	文本
	OpcodeBinary          Opcode = 0x2 // 	二级制
	OpcodeCloseConnection Opcode = 0x8 // 	关闭
	OpcodePing            Opcode = 0x9 // 	心跳探测
	OpcodePong            Opcode = 0xA //	心跳回应
)

// 判断操作码是否为数据帧
func (c Opcode) isDataFrame() bool {
	return c <= OpcodeBinary
}

type CloseError struct {
	Code   uint16 // 关闭代码
	Reason []byte // 关闭原因
}

// Error 返回关闭错误的描述
func (c *CloseError) Error() string {
	return fmt.Sprintf("gws: connection closed, code=%d, reason=%s", c.Code, string(c.Reason))
}

var (
	errEmpty = errors.New("")

	// ErrUnauthorized 未通过鉴权认证
	ErrUnauthorized = errors.New("gws: unauthorized")

	// ErrHandshake 握手错误, 请求头未通过校验
	ErrHandshake = errors.New("gws: handshake error")

	// ErrCompressionNegotiation 压缩拓展协商失败, 请尝试关闭压缩
	ErrCompressionNegotiation = errors.New("gws: invalid compression negotiation")

	// ErrSubprotocolNegotiation 子协议协商失败
	ErrSubprotocolNegotiation = errors.New("gws: sub-protocol negotiation failed")

	// ErrTextEncoding 文本消息编码错误(必须是utf8编码)
	ErrTextEncoding = errors.New("gws: invalid text encoding")

	// ErrMessageTooLarge 消息体积过大
	ErrMessageTooLarge = errors.New("gws: message too large")

	// ErrConnClosed 连接已关闭
	ErrConnClosed = net.ErrClosed

	// ErrUnsupportedProtocol 不支持的网络协议
	ErrUnsupportedProtocol = errors.New("gws: unsupported protocol")
)

// Event WebSocket 事件处理器
type Event interface {
	// OnOpen 连接建立事件
	OnOpen(socket *Conn)

	// OnClose 关闭事件
	// 收到对端关闭帧或IO错误导致主动断连; 前者 err 可断言为 *CloseError
	OnClose(socket *Conn, err error)

	// OnPing 心跳探测事件
	OnPing(socket *Conn, payload []byte)

	// OnPong 心跳响应事件
	OnPong(socket *Conn, payload []byte)

	// OnMessage 消息事件
	// 若开启 ParallelEnabled, 会并行调用且无 recover 保护
	OnMessage(socket *Conn, message *Message)
}

// BuiltinEventHandler 内置事件处理器
type BuiltinEventHandler struct{}

// OnOpen 默认连接建立事件回调
func (b BuiltinEventHandler) OnOpen(socket *Conn) {}

// OnClose 默认连接关闭事件回调
func (b BuiltinEventHandler) OnClose(socket *Conn, err error) {}

// OnPing 默认心跳探测事件回调, 自动回复 Pong 帧
func (b BuiltinEventHandler) OnPing(socket *Conn, payload []byte) { _ = socket.WritePong(payload) }

// OnPong 默认心跳响应事件回调
func (b BuiltinEventHandler) OnPong(socket *Conn, payload []byte) {}

// OnMessage 默认消息事件回调
func (b BuiltinEventHandler) OnMessage(socket *Conn, message *Message) {}

type frameHeader [frameHeaderSize]byte

// GetFIN 返回 FIN 位
func (c *frameHeader) GetFIN() bool {
	return ((*c)[0] >> 7) == 1
}

// GetRSV1 返回 RSV1 位
func (c *frameHeader) GetRSV1() bool {
	return ((*c)[0] << 1 >> 7) == 1
}

// GetRSV2 返回 RSV2 位
func (c *frameHeader) GetRSV2() bool {
	return ((*c)[0] << 2 >> 7) == 1
}

// GetRSV3 返回 RSV3 位
func (c *frameHeader) GetRSV3() bool {
	return ((*c)[0] << 3 >> 7) == 1
}

// GetOpcode 返回操作码
func (c *frameHeader) GetOpcode() Opcode {
	return Opcode((*c)[0] << 4 >> 4)
}

// GetMask 返回掩码位
func (c *frameHeader) GetMask() bool {
	return ((*c)[1] >> 7) == 1
}

// GetLengthCode 返回长度代码
func (c *frameHeader) GetLengthCode() uint8 {
	return (*c)[1] << 1 >> 1
}

// SetMask 设置 Mask 位为 1
func (c *frameHeader) SetMask() {
	(*c)[1] |= uint8(128)
}

// SetLength 设置帧长度并返回偏移量
func (c *frameHeader) SetLength(n uint64) (offset int) {
	if n <= internal.ThresholdV1 {
		(*c)[1] += uint8(n)
		return 0
	} else if n <= internal.ThresholdV2 {
		(*c)[1] += 126
		binary.BigEndian.PutUint16((*c)[2:4], uint16(n))
		return 2
	} else {
		(*c)[1] += 127
		binary.BigEndian.PutUint64((*c)[2:10], n)
		return 8
	}
}

// SetMaskKey 设置掩码键
func (c *frameHeader) SetMaskKey(offset int, key [4]byte) {
	copy((*c)[offset:offset+4], key[0:])
}

// GenerateHeader 生成帧头
// 客户端帧必须携带新鲜的随机掩码键, 由调用方通过连接自身的随机源生成 (见 Conn.nextMaskKey)
func (c *frameHeader) GenerateHeader(isServer bool, fin bool, compress bool, opcode Opcode, length int, maskNum uint32) (headerLength int, maskBytes []byte) {
	headerLength = 2
	var b0 = uint8(opcode)
	if fin {
		b0 += 128
	}
	if compress {
		b0 += 64
	}
	(*c)[0] = b0
	headerLength += c.SetLength(uint64(length))

	if !isServer {
		(*c)[1] |= 128
		binary.LittleEndian.PutUint32((*c)[headerLength:headerLength+4], maskNum)
		maskBytes = (*c)[headerLength : headerLength+4]
		headerLength += 4
	}
	return
}

// Parse 解析协议头, 最多14字节, 返回payload长度
func (c *frameHeader) Parse(reader io.Reader) (int, error) {
	if err := internal.ReadN(reader, (*c)[0:2]); err != nil {
		return 0, err
	}

	var payloadLength = 0
	var lengthCode = c.GetLengthCode()
	switch lengthCode {
	case 126:
		if err := internal.ReadN(reader, (*c)[2:4]); err != nil {
			return 0, err
		}
		payloadLength = int(binary.BigEndian.Uint16((*c)[2:4]))

	case 127:
		if err := internal.ReadN(reader, (*c)[2:10]); err != nil {
			return 0, err
		}
		// RFC6455 §5.2: 64位长度最高位必须为0, 且不得超过 math.MaxInt, 否则转int会得到负数
		var n = binary.BigEndian.Uint64((*c)[2:10])
		if n > uint64(math.MaxInt) {
			return 0, internal.CloseProtocolError
		}
		payloadLength = int(n)
	default:
		payloadLength = int(lengthCode)
	}

	var maskOn = c.GetMask()
	if maskOn {
		if err := internal.ReadN(reader, (*c)[10:14]); err != nil {
			return 0, err
		}
	}

	return payloadLength, nil
}

// GetMaskKey 返回掩码键
func (c *frameHeader) GetMaskKey() []byte {
	return (*c)[10:14]
}

type Message struct {
	compressed bool        // 是否压缩
	Opcode     Opcode      // 操作码
	Data       *bytes.Buffer // 消息内容
}

// Read 从消息读取数据到 p
func (c *Message) Read(p []byte) (n int, err error) {
	return c.Data.Read(p)
}

// Bytes 返回消息数据的字节切片
func (c *Message) Bytes() []byte {
	return c.Data.Bytes()
}

// Close 关闭消息并回收资源
func (c *Message) Close() error {
	if c.Data == nil {
		return nil
	}
	binaryPool.Put(c.Data)
	c.recycle()
	return nil
}

func (c *Message) recycle() {
	c.compressed = false
	c.Opcode = 0
	c.Data = nil
	messagePool.Put(c)
}

type continuationFrame struct {
	initialized bool          // 是否已初始化
	compressed  bool          // 是否压缩
	opcode      Opcode        // 操作码
	buffer      *bytes.Buffer // 缓冲区
}

// reset 重置延续帧状态
func (c *continuationFrame) reset() {
	c.initialized = false
	c.compressed = false
	c.opcode = 0
	c.buffer = nil
}

// Logger 日志接口
type Logger interface {
	// Error 打印错误日志
	Error(v ...any)
}

type stdLogger struct{}

// Error 打印错误日志
func (c *stdLogger) Error(v ...any) {
	log.Println(v...)
}

// Recovery 异常恢复并记录错误信息
func Recovery(logger Logger) {
	if e := recover(); e != nil {
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		msg := *(*string)(unsafe.Pointer(&buf))
		logger.Error("fatal error:", e, msg)
	}
}
