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

type Opcode uint8

const (
	OpcodeContinuation    Opcode = 0x0
	OpcodeText            Opcode = 0x1
	OpcodeBinary          Opcode = 0x2
	OpcodeCloseConnection Opcode = 0x8
	OpcodePing            Opcode = 0x9
	OpcodePong            Opcode = 0xA
)

func (c Opcode) isDataFrame() bool {
	return c <= OpcodeBinary
}

type CloseError struct {
	Code   uint16
	Reason []byte
}

func (c *CloseError) Error() string {
	return fmt.Sprintf("gws: connection closed, code=%d, reason=%s", c.Code, string(c.Reason))
}

var (
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

type BuiltinEventHandler struct{}

func (b BuiltinEventHandler) OnOpen(socket *Conn) {}

func (b BuiltinEventHandler) OnClose(socket *Conn, err error) {}

func (b BuiltinEventHandler) OnPing(socket *Conn, payload []byte) { _ = socket.WritePong(nil) }

func (b BuiltinEventHandler) OnPong(socket *Conn, payload []byte) {}

func (b BuiltinEventHandler) OnMessage(socket *Conn, message *Message) {}

type frameHeader [frameHeaderSize]byte

func (c *frameHeader) GetFIN() bool {
	return ((*c)[0] >> 7) == 1
}

func (c *frameHeader) GetRSV1() bool {
	return ((*c)[0] << 1 >> 7) == 1
}

func (c *frameHeader) GetRSV2() bool {
	return ((*c)[0] << 2 >> 7) == 1
}

func (c *frameHeader) GetRSV3() bool {
	return ((*c)[0] << 3 >> 7) == 1
}

func (c *frameHeader) GetOpcode() Opcode {
	return Opcode((*c)[0] << 4 >> 4)
}

func (c *frameHeader) GetMask() bool {
	return ((*c)[1] >> 7) == 1
}

func (c *frameHeader) GetLengthCode() uint8 {
	return (*c)[1] << 1 >> 1
}

func (c *frameHeader) SetMask() {
	(*c)[1] |= uint8(128)
}

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

func (c *frameHeader) SetMaskKey(offset int, key [4]byte) {
	copy((*c)[offset:offset+4], key[0:])
}

// GenerateHeader generate frame header for writing
// 可以考虑每个客户端连接带一个随机数发生器
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

// Parse 解析完整协议头, 最多14byte, 返回payload长度
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

// GetMaskKey parser把maskKey放到了末尾
func (c *frameHeader) GetMaskKey() []byte {
	return (*c)[10:14]
}

type Message struct {
	// 是否压缩
	compressed bool

	// 操作码
	Opcode Opcode

	// 消息内容
	Data *bytes.Buffer
}

func (c *Message) Read(p []byte) (n int, err error) {
	return c.Data.Read(p)
}

func (c *Message) Bytes() []byte {
	return c.Data.Bytes()
}

// Close recycle buffer
func (c *Message) Close() error {
	binaryPool.Put(c.Data)
	c.Data = nil
	return nil
}

type continuationFrame struct {
	initialized bool
	compressed  bool
	opcode      Opcode
	buffer      *bytes.Buffer
}

func (c *continuationFrame) reset() {
	c.initialized = false
	c.compressed = false
	c.opcode = 0
	c.buffer = nil
}

type Logger interface {
	Error(v ...any)
}

type stdLogger struct{}

func (c *stdLogger) Error(v ...any) {
	log.Println(v...)
}

func Recovery(logger Logger) {
	if e := recover(); e != nil {
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		msg := *(*string)(unsafe.Pointer(&buf))
		logger.Error("fatal error:", e, msg)
	}
}
