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

// 定义帧头的大小常量
// Define a constant for the frame header size
const frameHeaderSize = 14

// 定义 Opcode 类型，底层类型为 uint8
// Define the Opcode type, which is an alias for uint8
type Opcode uint8

// 定义各种操作码常量
// Define constants for various opcodes
const (
	// 继续帧操作码
	// Continuation frame opcode
	OpcodeContinuation Opcode = 0x0

	// 文本帧操作码
	// Text frame opcode
	OpcodeText Opcode = 0x1

	// 二进制帧操作码
	// Binary frame opcode
	OpcodeBinary Opcode = 0x2

	// 关闭连接操作码
	// Close connection opcode
	OpcodeCloseConnection Opcode = 0x8

	// Ping 操作码
	// Ping opcode
	OpcodePing Opcode = 0x9

	// Pong 操作码
	// Pong opcode
	OpcodePong Opcode = 0xA
)

// isDataFrame 方法判断操作码是否为数据帧
// The isDataFrame method checks if the opcode is a data frame
func (c Opcode) isDataFrame() bool {
	// 如果操作码小于等于二进制帧操作码，则返回 true
	// Return true if the opcode is less than or equal to the binary frame opcode
	return c <= OpcodeBinary
}

// 定义 CloseError 类型，包含关闭代码和原因
// Define the CloseError type, which includes a close code and a reason
type CloseError struct {
	// 关闭代码，表示关闭连接的原因
	// Close code, indicating the reason for closing the connection
	Code uint16

	// 关闭原因，详细描述关闭的原因
	// Close reason, providing a detailed description of the closure
	Reason []byte
}

// Error 方法返回关闭错误的描述
// The Error method returns a description of the close error
func (c *CloseError) Error() string {
	// 返回格式化的错误信息，包含关闭代码和原因
	// Return a formatted error message that includes the close code and reason
	return fmt.Sprintf("gws: connection closed, code=%d, reason=%s", c.Code, string(c.Reason))
}

var (
	// ErrEmpty 空错误
	// Empty error
	errEmpty = errors.New("")

	// ErrUnauthorized 未通过鉴权认证
	// Failure to pass forensic authentication
	ErrUnauthorized = errors.New("unauthorized")

	// ErrHandshake 握手错误, 请求头未通过校验
	// Handshake error, request header does not pass checksum.
	ErrHandshake = errors.New("handshake error")

	// ErrCompressionNegotiation 压缩拓展协商失败, 请尝试关闭压缩
	// Compression extension negotiation failed, please try to disable compression.
	ErrCompressionNegotiation = errors.New("invalid compression negotiation")

	// ErrSubprotocolNegotiation 子协议协商失败
	// Sub-protocol negotiation failed
	ErrSubprotocolNegotiation = errors.New("sub-protocol negotiation failed")

	// ErrTextEncoding 文本消息编码错误(必须是utf8编码)
	// Text message encoding error (must be utf8)
	ErrTextEncoding = errors.New("invalid text encoding")

	// ErrConnClosed 连接已关闭
	// Connection closed
	ErrConnClosed = net.ErrClosed

	// ErrUnsupportedProtocol 不支持的网络协议
	// Unsupported network protocols
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
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

// BuiltinEventHandler 是一个内置的事件处理器结构体
// BuiltinEventHandler is a built-in event handler struct
type BuiltinEventHandler struct{}

// OnOpen 在连接打开时调用
// OnOpen is called when the connection is opened
func (b BuiltinEventHandler) OnOpen(socket *Conn) {}

// OnClose 在连接关闭时调用
// OnClose is called when the connection is closed
func (b BuiltinEventHandler) OnClose(socket *Conn, err error) {}

// OnPing 在接收到 Ping 帧时调用
// OnPing is called when a Ping frame is received
func (b BuiltinEventHandler) OnPing(socket *Conn, payload []byte) {
	// 发送 Pong 帧作为响应
	// Send a Pong frame in response
	_ = socket.WritePong(nil)
}

// OnPong 在接收到 Pong 帧时调用
// OnPong is called when a Pong frame is received
func (b BuiltinEventHandler) OnPong(socket *Conn, payload []byte) {}

// OnMessage 在接收到消息时调用
// OnMessage is called when a message is received
func (b BuiltinEventHandler) OnMessage(socket *Conn, message *Message) {}

// 定义帧头类型，大小为 frameHeaderSize 的字节数组
// Define the frameHeader type, which is an array of bytes with size frameHeaderSize
type frameHeader [frameHeaderSize]byte

// GetFIN 方法返回 FIN 位的值
// The GetFIN method returns the value of the FIN bit
func (c *frameHeader) GetFIN() bool {
	// 通过右移 7 位获取第一个字节的最高位
	// Get the highest bit of the first byte by shifting right 7 bits
	return ((*c)[0] >> 7) == 1
}

// GetRSV1 方法返回 RSV1 位的值
// The GetRSV1 method returns the value of the RSV1 bit
func (c *frameHeader) GetRSV1() bool {
	// 通过左移 1 位再右移 7 位获取第一个字节的第二高位
	// Get the second highest bit of the first byte by shifting left 1 bit and then right 7 bits
	return ((*c)[0] << 1 >> 7) == 1
}

// GetRSV2 方法返回 RSV2 位的值
// The GetRSV2 method returns the value of the RSV2 bit
func (c *frameHeader) GetRSV2() bool {
	// 通过左移 2 位再右移 7 位获取第一个字节的第三高位
	// Get the third highest bit of the first byte by shifting left 2 bits and then right 7 bits
	return ((*c)[0] << 2 >> 7) == 1
}

// GetRSV3 方法返回 RSV3 位的值
// The GetRSV3 method returns the value of the RSV3 bit
func (c *frameHeader) GetRSV3() bool {
	// 通过左移 3 位再右移 7 位获取第一个字节的第四高位
	// Get the fourth highest bit of the first byte by shifting left 3 bits and then right 7 bits
	return ((*c)[0] << 3 >> 7) == 1
}

// GetOpcode 方法返回操作码
// The GetOpcode method returns the opcode
func (c *frameHeader) GetOpcode() Opcode {
	// 通过左移 4 位再右移 4 位获取第一个字节的低 4 位
	// Get the lowest 4 bits of the first byte by shifting left 4 bits and then right 4 bits
	return Opcode((*c)[0] << 4 >> 4)
}

// GetMask 方法返回 Mask 位的值
// The GetMask method returns the value of the Mask bit
func (c *frameHeader) GetMask() bool {
	// 通过右移 7 位获取第二个字节的最高位
	// Get the highest bit of the second byte by shifting right 7 bits
	return ((*c)[1] >> 7) == 1
}

// GetLengthCode 方法返回长度代码
// The GetLengthCode method returns the length code
func (c *frameHeader) GetLengthCode() uint8 {
	// 通过左移 1 位再右移 1 位获取第二个字节的低 7 位
	// Get the lowest 7 bits of the second byte by shifting left 1 bit and then right 1 bit
	return (*c)[1] << 1 >> 1
}

// SetMask 方法设置 Mask 位为 1
// The SetMask method sets the Mask bit to 1
func (c *frameHeader) SetMask() {
	// 将第二个字节的最高位置为 1
	// Set the highest bit of the second byte to 1
	(*c)[1] |= uint8(128)
}

// SetLength 方法设置帧的长度，并返回偏移量
// The SetLength method sets the frame length and returns the offset
func (c *frameHeader) SetLength(n uint64) (offset int) {
	// 如果长度小于等于 ThresholdV1
	// If the length is less than or equal to ThresholdV1
	if n <= internal.ThresholdV1 {
		// 将长度直接设置到帧头的第二个字节
		// Set the length directly in the second byte of the frame header
		(*c)[1] += uint8(n)

		// 返回 0 偏移量
		// Return 0 offset
		return 0

	} else if n <= internal.ThresholdV2 {
		// 如果长度小于等于 ThresholdV2
		// If the length is less than or equal to ThresholdV2
		// 将长度代码设置为 126
		// Set the length code to 126
		(*c)[1] += 126

		// 将长度的值存储在帧头的第 3 到第 4 字节
		// Store the length value in the 3rd to 4th bytes of the frame header
		binary.BigEndian.PutUint16((*c)[2:4], uint16(n))

		// 返回 2 偏移量
		// Return 2 offset
		return 2

	} else {
		// 如果长度大于 ThresholdV2
		// If the length is greater than ThresholdV2
		// 将长度代码设置为 127
		// Set the length code to 127
		(*c)[1] += 127

		// 将长度的值存储在帧头的第 3 到第 10 字节
		// Store the length value in the 3rd to 10th bytes of the frame header
		binary.BigEndian.PutUint64((*c)[2:10], n)

		// 返回 8 偏移量
		// Return 8 offset
		return 8
	}
}

// SetMaskKey 方法设置掩码键
// The SetMaskKey method sets the mask key
func (c *frameHeader) SetMaskKey(offset int, key [4]byte) {
	// 将掩码键复制到帧头的指定偏移量位置
	// Copy the mask key to the specified offset in the frame header
	copy((*c)[offset:offset+4], key[0:])
}

// GenerateHeader 生成用于写入的帧头
// GenerateHeader generates a frame header for writing
// 可以考虑每个客户端连接带一个随机数发生器
// Consider having a random number generator for each client connection
func (c *frameHeader) GenerateHeader(isServer bool, fin bool, compress bool, opcode Opcode, length int) (headerLength int, maskBytes []byte) {
	// 初始化帧头长度为 2
	// Initialize the header length to 2
	headerLength = 2

	// 初始化第一个字节为操作码
	// Initialize the first byte with the opcode
	var b0 = uint8(opcode)

	// 如果是最后一帧，设置 FIN 位
	// If this is the final frame, set the FIN bit
	if fin {
		b0 += 128
	}

	// 如果需要压缩，设置压缩位
	// If compression is needed, set the compression bit
	if compress {
		b0 += 64
	}

	// 设置帧头的第一个字节
	// Set the first byte of the frame header
	(*c)[0] = b0

	// 设置帧的长度，并增加帧头长度
	// Set the frame length and increase the header length
	headerLength += c.SetLength(uint64(length))

	// 如果不是服务器，设置掩码位并生成掩码键
	// If not a server, set the mask bit and generate a mask key
	if !isServer {
		// 设置掩码位
		// Set the mask bit
		(*c)[1] |= 128

		// 生成一个随机掩码键
		// Generate a random mask key
		maskNum := internal.AlphabetNumeric.Uint32()

		// 将掩码键写入帧头
		// Write the mask key into the frame header
		binary.LittleEndian.PutUint32((*c)[headerLength:headerLength+4], maskNum)

		// 设置掩码字节
		// Set the mask bytes
		maskBytes = (*c)[headerLength : headerLength+4]

		// 增加帧头长度
		// Increase the header length
		headerLength += 4
	}

	// 无效代码
	// Invalid code
	return
}

// Parse 解析完整协议头, 最多14字节, 返回payload长度
// Parse parses the complete protocol header, up to 14 bytes, and returns the payload length
func (c *frameHeader) Parse(reader io.Reader) (int, error) {
	// 读取前两个字节到帧头
	// Read the first two bytes into the frame header
	if err := internal.ReadN(reader, (*c)[0:2]); err != nil {
		return 0, err
	}

	// 初始化 payload 长度为 0
	// Initialize payload length to 0
	var payloadLength = 0
	// 获取长度代码
	// Get the length code
	var lengthCode = c.GetLengthCode()

	// 根据长度代码解析 payload 长度
	// Parse the payload length based on the length code
	switch lengthCode {
	case 126:
		// 如果长度代码是 126，读取接下来的两个字节
		// If the length code is 126, read the next two bytes
		if err := internal.ReadN(reader, (*c)[2:4]); err != nil {
			return 0, err
		}

		// 将这两个字节转换为 payload 长度
		// Convert these two bytes to the payload length
		payloadLength = int(binary.BigEndian.Uint16((*c)[2:4]))

	case 127:
		// 如果长度代码是 127，读取接下来的八个字节
		// If the length code is 127, read the next eight bytes
		if err := internal.ReadN(reader, (*c)[2:10]); err != nil {
			return 0, err
		}

		// 将这八个字节转换为 payload 长度
		// Convert these eight bytes to the payload length
		payloadLength = int(binary.BigEndian.Uint64((*c)[2:10]))

	default:
		// 否则，长度代码就是 payload 长度
		// Otherwise, the length code is the payload length
		payloadLength = int(lengthCode)
	}

	// 检查是否有掩码
	// Check if there is a mask
	var maskOn = c.GetMask()
	if maskOn {
		// 如果有掩码，读取接下来的四个字节
		// If there is a mask, read the next four bytes
		if err := internal.ReadN(reader, (*c)[10:14]); err != nil {
			return 0, err
		}
	}

	// 返回 payload 长度
	// Return the payload length
	return payloadLength, nil
}

// GetMaskKey 方法返回掩码键
// The GetMaskKey method returns the mask key
// parser把maskKey放到了末尾
// The parser places the mask key at the end
func (c *frameHeader) GetMaskKey() []byte {
	// 返回帧头中第 10 到第 14 字节作为掩码键
	// Return the 10th to 14th bytes of the frame header as the mask key
	return (*c)[10:14]
}

// Message 结构体表示一个消息
// The Message struct represents a message
type Message struct {
	// 是否压缩
	// Indicates if the message is compressed
	compressed bool

	// 操作码
	// The opcode of the message
	Opcode Opcode

	// 消息内容
	// The content of the message
	Data *bytes.Buffer
}

// Read 从消息中读取数据到给定的字节切片 p 中
// Read reads data from the message into the given byte slice p
func (c *Message) Read(p []byte) (n int, err error) {
	// 从消息的数据缓冲区中读取数据
	// Read data from the message's data buffer
	return c.Data.Read(p)
}

// Bytes 返回消息的数据缓冲区的字节切片
// Bytes returns the byte slice of the message's data buffer
func (c *Message) Bytes() []byte {
	return c.Data.Bytes()
}

// Close 回收缓冲区
// Close recycles the buffer
func (c *Message) Close() error {
	// 将数据缓冲区放回缓冲池
	// Put the data buffer back into the buffer pool
	binaryPool.Put(c.Data)

	// 将数据缓冲区设置为 nil
	// Set the data buffer to nil
	c.Data = nil

	// 返回 nil 表示没有错误
	// Return nil to indicate no error
	return nil
}

// continuationFrame 结构体表示一个延续帧
// The continuationFrame struct represents a continuation frame
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

// reset 方法重置延续帧的状态
// The reset method resets the state of the continuation frame
func (c *continuationFrame) reset() {
	// 将 initialized 设置为 false
	// Set initialized to false
	c.initialized = false

	// 将 compressed 设置为 false
	// Set compressed to false
	c.compressed = false

	// 将 opcode 设置为 0
	// Set opcode to 0
	c.opcode = 0

	// 将 buffer 设置为 nil
	// Set buffer to nil
	c.buffer = nil
}

// Logger 接口定义了一个日志记录器
// The Logger interface defines a logger
type Logger interface {
	// Error 方法记录错误信息
	// The Error method logs error messages
	Error(v ...any)
}

// stdLogger 结构体实现了 Logger 接口
// The stdLogger struct implements the Logger interface
type stdLogger struct{}

// Error 方法实现了 Logger 接口的 Error 方法，使用标准日志记录错误信息
// The Error method implements the Logger interface's Error method, using the standard log to record error messages
func (c *stdLogger) Error(v ...any) {
	log.Println(v...)
}

// Recovery 函数用于从 panic 中恢复，并记录错误信息
// The Recovery function is used to recover from a panic and log error messages
func Recovery(logger Logger) {
	// 如果有 panic 发生
	// If a panic occurs
	if e := recover(); e != nil {
		// 定义缓冲区大小为 64KB
		// Define the buffer size as 64KB
		const size = 64 << 10

		// 创建缓冲区
		// Create a buffer
		buf := make([]byte, size)

		// 获取当前 goroutine 的堆栈信息
		// Get the stack trace of the current goroutine
		buf = buf[:runtime.Stack(buf, false)]

		// 将缓冲区转换为字符串
		// Convert the buffer to a string
		msg := *(*string)(unsafe.Pointer(&buf))

		// 记录错误信息，包括 panic 的值和堆栈信息
		// Log the error message, including the panic value and stack trace
		logger.Error("fatal error:", e, msg)
	}
}
