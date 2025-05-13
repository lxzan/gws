package gws

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
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
// Checks if the opcode is a data frame
func (c Opcode) isDataFrame() bool {
	return c <= OpcodeBinary
}

type CloseError struct {
	// 关闭代码，表示关闭连接的原因
	// Close code, indicating the reason for closing the connection
	Code uint16

	// 关闭原因，详细描述关闭的原因
	// Close reason, providing a detailed description of the closure
	Reason []byte
}

// Error 关闭错误的描述
// Returns a description of the close error
func (c *CloseError) Error() string {
	return fmt.Sprintf("gws: connection closed, code=%d, reason=%s", c.Code, string(c.Reason))
}

var (
	errEmpty = errors.New("")

	// ErrUnauthorized 未通过鉴权认证
	// Failure to pass forensic authentication
	ErrUnauthorized = errors.New("gws: unauthorized")

	// ErrHandshake 握手错误, 请求头未通过校验
	// Handshake error, request header does not pass checksum.
	ErrHandshake = errors.New("gws: handshake error")

	// ErrCompressionNegotiation 压缩拓展协商失败, 请尝试关闭压缩
	// Compression extension negotiation failed, please try to disable compression.
	ErrCompressionNegotiation = errors.New("gws: invalid compression negotiation")

	// ErrSubprotocolNegotiation 子协议协商失败
	// Sub-protocol negotiation failed
	ErrSubprotocolNegotiation = errors.New("gws: sub-protocol negotiation failed")

	// ErrTextEncoding 文本消息编码错误(必须是utf8编码)
	// Text message encoding error (must be utf8)
	ErrTextEncoding = errors.New("gws: invalid text encoding")

	// ErrMessageTooLarge 消息体积过大
	// message is too large
	ErrMessageTooLarge = errors.New("gws: message too large")

	// ErrConnClosed 连接已关闭
	// Connection closed
	ErrConnClosed = net.ErrClosed

	// ErrUnsupportedProtocol 不支持的网络协议
	// Unsupported network protocols
	ErrUnsupportedProtocol = errors.New("gws: unsupported protocol")
)

type Event interface {
	// OnOpen 建立连接事件
	// WebSocket connection was successfully established
	OnOpen(socket *Conn)

	// OnClose 关闭事件
	// 接收到了网络连接另一端发送的关闭帧, 或者IO过程中出现错误主动断开连接
	// 如果是前者, err可以断言为*CloseError
	// Received a close frame from the other end of the network connection, or disconnected voluntarily due to an error in the IO process
	// In the former case, err can be asserted as *CloseError
	OnClose(socket *Conn, err error)

	// OnPing 心跳探测事件
	// Received a ping frame
	OnPing(socket *Conn, payload []byte)

	// OnPong 心跳响应事件
	// Received a pong frame
	OnPong(socket *Conn, payload []byte)

	// OnMessage 消息事件
	// 如果开启了ParallelEnabled, 会并行地调用OnMessage; 没有做recover处理.
	// If ParallelEnabled is enabled, OnMessage is called in parallel. No recover is done.
	OnMessage(socket *Conn, message *Message)
}

type BuiltinEventHandler struct{}

func (b BuiltinEventHandler) OnOpen(socket *Conn) {}

func (b BuiltinEventHandler) OnClose(socket *Conn, err error) {}

func (b BuiltinEventHandler) OnPing(socket *Conn, payload []byte) { _ = socket.WritePong(nil) }

func (b BuiltinEventHandler) OnPong(socket *Conn, payload []byte) {}

func (b BuiltinEventHandler) OnMessage(socket *Conn, message *Message) {}

type frameHeader [frameHeaderSize]byte

// GetFIN 返回 FIN 位的值
// Returns the value of the FIN bit
func (c *frameHeader) GetFIN() bool {
	return ((*c)[0] >> 7) == 1
}

// GetRSV1 返回 RSV1 位的值
// Returns the value of the RSV1 bit
func (c *frameHeader) GetRSV1() bool {
	return ((*c)[0] << 1 >> 7) == 1
}

// GetRSV2 返回 RSV2 位的值
// Returns the value of the RSV2 bit
func (c *frameHeader) GetRSV2() bool {
	return ((*c)[0] << 2 >> 7) == 1
}

// GetRSV3 返回 RSV3 位的值
// Returns the value of the RSV3 bit
func (c *frameHeader) GetRSV3() bool {
	return ((*c)[0] << 3 >> 7) == 1
}

// GetOpcode 返回操作码
// Returns the opcode
func (c *frameHeader) GetOpcode() Opcode {
	return Opcode((*c)[0] << 4 >> 4)
}

// GetMask 返回掩码
// Returns the value of the mask bytes
func (c *frameHeader) GetMask() bool {
	return ((*c)[1] >> 7) == 1
}

// GetLengthCode 返回长度代码
// Returns the length code
func (c *frameHeader) GetLengthCode() uint8 {
	return (*c)[1] << 1 >> 1
}

// SetMask 设置 Mask 位为 1
// Sets the Mask bit to 1
func (c *frameHeader) SetMask() {
	(*c)[1] |= uint8(128)
}

// SetLength 设置帧的长度，并返回偏移量
// Sets the frame length and returns the offset
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

// SetMaskKey 设置掩码
// Sets the mask
func (c *frameHeader) SetMaskKey(offset int, key [4]byte) {
	copy((*c)[offset:offset+4], key[0:])
}

// GenerateHeader 生成帧头
// Generates a frame header
// 可以考虑每个客户端连接带一个随机数发生器
// Consider having a random number generator for each client connection
func (c *frameHeader) GenerateHeader(isServer bool, fin bool, compress bool, opcode Opcode, length int) (headerLength int, maskBytes []byte) {
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
		maskNum := internal.AlphabetNumeric.Uint32()
		binary.LittleEndian.PutUint32((*c)[headerLength:headerLength+4], maskNum)
		maskBytes = (*c)[headerLength : headerLength+4]
		headerLength += 4
	}
	return
}

// Parse 解析完整协议头, 最多14字节, 返回payload长度
// Parses the complete protocol header, up to 14 bytes, and returns the payload length
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
		payloadLength = int(binary.BigEndian.Uint64((*c)[2:10]))
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

// GetMaskKey 返回掩码
// Returns the mask
func (c *frameHeader) GetMaskKey() []byte {
	return (*c)[10:14]
}

type Message struct {
	// 是否压缩
	// if the message is compressed
	compressed bool

	// 操作码
	// opcode of the message
	Opcode Opcode

	// 消息内容
	// content of the message
	Data *bytes.Buffer
}

// Read 从消息中读取数据到给定的字节切片 p 中
// Reads data from the message into the given byte slice p
func (c *Message) Read(p []byte) (n int, err error) {
	return c.Data.Read(p)
}

// Bytes 返回消息的数据缓冲区的字节切片
// Returns the byte slice of the message's data buffer
func (c *Message) Bytes() []byte {
	return c.Data.Bytes()
}

// Close 关闭消息, 回收资源
// Close message, recycling resources
func (c *Message) Close() error {
	binaryPool.Put(c.Data)
	c.Data = nil
	return nil
}

type continuationFrame struct {
	// 是否已初始化
	// Indicates if the frame is initialized
	initialized bool

	// 是否压缩
	// Indicates if the frame is compressed
	compressed bool

	// 操作码
	// The opcode of the frame
	opcode Opcode

	// 缓冲区
	// The buffer for the frame data
	buffer *bytes.Buffer
}

// 重置延续帧的状态
// Resets the state of the continuation frame
func (c *continuationFrame) reset() {
	c.initialized = false
	c.compressed = false
	c.opcode = 0
	c.buffer = nil
}

// Logger 日志接口
// Logger interface
type Logger interface {
	// Error 打印错误日志
	// Printing the error log
	Error(v ...any)
}

// 标准日志库
// Standard Log Library
type stdLogger struct{}

// Error 打印错误日志
// Printing the error log
func (c *stdLogger) Error(v ...any) {
	log.Println(v...)
}

// Recovery 异常恢复，并记录错误信息
// Exception recovery with logging of error messages
func Recovery(logger Logger) {
	if e := recover(); e != nil {
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		msg := *(*string)(unsafe.Pointer(&buf))
		logger.Error("fatal error:", e, msg)
	}
}
