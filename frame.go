package gws

import (
	"encoding/binary"
	"github.com/lxzan/gws/internal"
)

func isDataFrame(code Opcode) bool {
	return code <= OpcodeBinary
}

type frameHeader [internal.FrameHeaderSize]byte

func (c *frameHeader) GetFIN() bool {
	return ((*c)[0] >> 7) == 1
}

func (c *frameHeader) GetRSV1() bool {
	return ((*c)[0] << 1 >> 7) == 1
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
