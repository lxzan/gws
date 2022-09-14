package internal

import "io"

func NewBytesBuffer(b []byte) *BytesBuffer {
	return &BytesBuffer{b: b}
}

type BytesBuffer struct {
	offset int
	b      []byte
}

func (c *BytesBuffer) Write(p []byte) (n int, err error) {
	var lenDst = len(c.b)
	var lenSrc = len(p)
	var end = cap(c.b)
	var capacity = end - lenDst

	if capacity >= lenSrc {
		c.b = c.b[:lenDst+lenSrc]
		copy(c.b[lenDst:], p)
		return lenSrc, nil
	}

	c.b = c.b[:end]
	copy(c.b[lenDst:end], p[:capacity])
	c.b = append(c.b, p[capacity:]...)
	return len(p), nil
}

func (c *BytesBuffer) Read(p []byte) (n int, err error) {
	var lenSrc = len(c.b) - c.offset
	var lenDst = len(p)
	if lenSrc == 0 {
		return 0, io.EOF
	}

	copy(p, c.b[c.offset:])
	if lenSrc <= lenDst {
		c.offset += lenSrc
		return lenSrc, nil
	}

	c.offset += lenDst
	return lenDst, nil
}

func (c *BytesBuffer) Reset() {
	c.b = c.b[:0]
}

func (c *BytesBuffer) Bytes() []byte {
	return c.b[c.offset:]
}
