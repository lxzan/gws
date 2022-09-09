package websocket

import (
	"encoding/binary"
	"github.com/lxzan/websocket/internal"
)

func isDataFrame(code Opcode) bool {
	return code <= Opcode_Binary
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
	(*c)[0] &= uint8(240)
	(*c)[0] += uint8(opcode)
}

func (c *frameHeader) SetMaskOn() {
	(*c)[1] |= internal.Bv7
}

func (c *frameHeader) SetLength7(length uint8) {
	(*c)[1] &= internal.Bv7
	(*c)[1] += length
}

func (c *frameHeader) SetLength16(length uint16) {
	binary.BigEndian.PutUint16((*c)[2:4], length)
}

func (c *frameHeader) SetLength64(length uint64) {
	binary.BigEndian.PutUint64((*c)[2:10], length)
}

func (c *frameHeader) SetMaskKey(offset int, key [4]byte) {
	start := 2 + offset
	copy((*c)[start:start+4], key[:4])
}

func genHeader(side uint8, opcode Opcode, firstFrame bool, lastFrame bool, enableCompress bool, contentLength uint64) (frameHeader, int) {
	var headerLength = 2
	var h = frameHeader{}
	if !firstFrame {
		opcode = Opcode_Continuation
	}
	if lastFrame {
		h.SetFIN()
	}
	if enableCompress {
		h.SetRSV1()
	}

	enableMask := side == clientSide && isDataFrame(opcode)
	if enableMask {
		h.SetMaskOn()
		headerLength += 4
	}
	h.SetOpcode(opcode)

	var offset = 0
	if contentLength <= internal.PayloadSizeLv1 {
		h.SetLength7(uint8(contentLength))
	} else if contentLength <= internal.PayloadSizeLv2 {
		h.SetLength7(126)
		h.SetLength16(uint16(contentLength))
		offset += 2
	} else {
		h.SetLength7(127)
		h.SetLength64(contentLength)
		offset += 8
	}

	headerLength += offset
	if enableMask {
		h.SetMaskKey(offset, internal.NewMaskKey())
	}

	return h, headerLength
}
