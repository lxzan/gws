package gws

import (
	"bytes"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
)

const frameHeaderSize = 14

var frameHeaderPadding = frameHeader{}

type Opcode uint8

const (
	OpcodeContinuation    Opcode = 0x0
	OpcodeText            Opcode = 0x1
	OpcodeBinary          Opcode = 0x2
	OpcodeCloseConnection Opcode = 0x8
	OpcodePing            Opcode = 0x9
	OpcodePong            Opcode = 0xA
)

func (c Opcode) IsDataFrame() bool {
	return c <= OpcodeBinary
}

// WebSocket Event
// one of onclose and onerror will be called once during the connection's lifetime.
// 在连接的生命周期中，onclose和onerror中的一个有且只有一次被调用.
type Event interface {
	// 建立连接事件
	OnOpen(socket *Conn)

	// 错误事件
	// IO出现错误, 协议错误, 压缩解压错误...
	OnError(socket *Conn, err error)

	// 关闭事件
	// 接收到了另一端发送的关闭帧
	OnClose(socket *Conn, code uint16, reason []byte)

	// 心跳探测事件
	OnPing(socket *Conn, payload []byte)

	// 心跳响应事件
	OnPong(socket *Conn, payload []byte)

	// 消息事件
	// 如果开启了AsyncReadEnabled, 可以在一个连接里面并行处理多个请求
	OnMessage(socket *Conn, message *Message)
}

type BuiltinEventHandler struct{}

func (b BuiltinEventHandler) OnOpen(socket *Conn) {}

func (b BuiltinEventHandler) OnError(socket *Conn, err error) {}

func (b BuiltinEventHandler) OnClose(socket *Conn, code uint16, reason []byte) {}

func (b BuiltinEventHandler) OnPing(socket *Conn, payload []byte) {}

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

// generate frame header for writing
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

// 解析完整协议头, 最多14byte, 返回payload长度
func (c *frameHeader) Parse(reader io.Reader) (int, error) {
	if err := internal.ReadN(reader, (*c)[0:2], 2); err != nil {
		return 0, err
	}

	var payloadLength = 0
	var lengthCode = c.GetLengthCode()
	switch lengthCode {
	case 126:
		if err := internal.ReadN(reader, (*c)[2:4], 2); err != nil {
			return 0, err
		}
		payloadLength = int(binary.BigEndian.Uint16((*c)[2:4]))

	case 127:
		if err := internal.ReadN(reader, (*c)[2:10], 8); err != nil {
			return 0, err
		}
		payloadLength = int(binary.BigEndian.Uint64((*c)[2:10]))
	default:
		payloadLength = int(lengthCode)
	}

	var maskOn = c.GetMask()
	if maskOn {
		if err := internal.ReadN(reader, (*c)[10:14], 4); err != nil {
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
	Opcode Opcode        // 帧状态码
	Data   *bytes.Buffer // 数据缓冲
}

func (c *Message) Read(p []byte) (n int, err error) {
	return c.Data.Read(p)
}

func (c *Message) Bytes() []byte {
	return c.Data.Bytes()
}

// Close recycle buffer
func (c *Message) Close() {
	_bpool.Put(c.Data)
	c.Data = nil
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
