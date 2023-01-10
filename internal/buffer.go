package internal

import "io"

type BufferInterface interface {
	io.ReadWriter
	Reset()
	Bytes() []byte
	Cap() int
	Len() int
}

/*
NewBuffer
io.CopyN会造成bytes.Buffer扩容, 所以我自己实现了Buffer
*/
func NewBuffer(b []byte) *Buffer {
	return &Buffer{b: b}
}

func NewBufferWithCap(n uint8) *Buffer {
	var buf = &Buffer{}
	if n > 0 {
		buf.b = make([]byte, 0, n)
	}
	return buf
}

type Buffer struct {
	offset int
	b      []byte
}

func (c *Buffer) Write(p []byte) (n int, err error) {
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

func (c *Buffer) Read(p []byte) (n int, err error) {
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

func (c *Buffer) Reset() {
	c.offset = 0
	c.b = c.b[:0]
}

func (c *Buffer) Bytes() []byte {
	return c.b[c.offset:]
}

func (c *Buffer) Cap() int {
	return cap(c.b)
}

func (c *Buffer) Len() int {
	return len(c.b) - c.offset
}

func (c *Buffer) Available() int {
	return cap(c.b) - len(c.b)
}
