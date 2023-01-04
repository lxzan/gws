package gws

import (
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"unicode/utf8"
)

type Message struct {
	closeCode CloseCode        // 关闭状态码
	opcode    Opcode           // 帧状态码
	buf       *internal.Buffer // 数据缓冲
}

func (c *Message) Read(p []byte) (n int, err error) {
	return c.buf.Read(p)
}

// Close recycle buffer
func (c *Message) Close() {
	_pool.Put(c.buf)
	c.buf = nil
}

// Code get close code
// only close frame has the code
func (c *Message) Code() CloseCode {
	return c.closeCode
}

// Typ get message type
func (c *Message) Typ() Opcode {
	return c.opcode
}

// Bytes get message content
func (c *Message) Bytes() []byte {
	return c.buf.Bytes()
}

func payloadValid(opcode Opcode, buf *internal.Buffer) bool {
	if buf.Len() == 0 && !(opcode == OpcodeCloseConnection || opcode == OpcodeText) {
		return true
	}
	return utf8.Valid(buf.Bytes())
}

func maskXOR(b []byte, key []byte) {
	var maskKey = binary.LittleEndian.Uint32(key)
	var key64 = uint64(maskKey)<<32 + uint64(maskKey)

	for len(b) >= 64 {
		v := binary.LittleEndian.Uint64(b)
		binary.LittleEndian.PutUint64(b, v^key64)
		v = binary.LittleEndian.Uint64(b[8:16])
		binary.LittleEndian.PutUint64(b[8:16], v^key64)
		v = binary.LittleEndian.Uint64(b[16:24])
		binary.LittleEndian.PutUint64(b[16:24], v^key64)
		v = binary.LittleEndian.Uint64(b[24:32])
		binary.LittleEndian.PutUint64(b[24:32], v^key64)
		v = binary.LittleEndian.Uint64(b[32:40])
		binary.LittleEndian.PutUint64(b[32:40], v^key64)
		v = binary.LittleEndian.Uint64(b[40:48])
		binary.LittleEndian.PutUint64(b[40:48], v^key64)
		v = binary.LittleEndian.Uint64(b[48:56])
		binary.LittleEndian.PutUint64(b[48:56], v^key64)
		v = binary.LittleEndian.Uint64(b[56:64])
		binary.LittleEndian.PutUint64(b[56:64], v^key64)
		b = b[64:]
	}

	for len(b) >= 8 {
		v := binary.LittleEndian.Uint64(b)
		binary.LittleEndian.PutUint64(b, v^key64)
		b = b[8:]
	}

	var n = len(b)
	for i := 0; i < n; i++ {
		idx := i & 3
		b[i] ^= key[idx]
	}
}

type frameHeader [internal.FrameHeaderSize]byte

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

func (c *frameHeader) SetFIN() {
	(*c)[0] |= internal.Bv7
}

func (c *frameHeader) SetRSV1() {
	(*c)[0] |= internal.Bv6
}

func (c *frameHeader) SetOpcode(opcode Opcode) {
	(*c)[0] += uint8(opcode)
}

func (c *frameHeader) SetMaskOn() {
	(*c)[1] |= internal.Bv7
}

func (c *frameHeader) SetLength(n uint64) (offset int) {
	if n <= internal.Lv1 {
		(*c)[1] += uint8(n)
		return 0
	} else if n <= internal.Lv4 {
		(*c)[1] += 126
		binary.BigEndian.PutUint16((*c)[2:4], uint16(n))
		return 2
	} else {
		(*c)[1] += 127
		binary.BigEndian.PutUint64((*c)[2:10], n)
		return 8
	}
}

func (c *frameHeader) SetMaskKey(offset int, key uint32) {
	binary.BigEndian.PutUint32((*c)[offset+2:offset+6], key)
}

// generate server side frame header for writing
// do not use mask
func (c *frameHeader) GenerateServerHeader(opcode Opcode, enableCompress bool, length int) int {
	var headerLength = 2
	c.SetFIN()
	if enableCompress {
		c.SetRSV1()
	}
	c.SetOpcode(opcode)
	headerLength += c.SetLength(uint64(length))
	return headerLength
}
