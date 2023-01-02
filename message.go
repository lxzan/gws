package gws

import (
	"github.com/lxzan/gws/internal"
)

type Message struct {
	err        error
	opcode     Opcode
	compressed bool
	dbuf       *internal.Buffer // 数据缓冲
	cbuf       *internal.Buffer // 解码器缓冲
}

func (c *Message) Read(p []byte) (n int, err error) {
	return c.dbuf.Read(p)
}

func (c *Message) Close() error {
	if c.dbuf != nil {
		_pool.Put(c.dbuf)
	}
	if c.cbuf != nil {
		_pool.Put(c.cbuf)
	}
	return nil
}

func (c *Message) Err() error {
	return c.err
}

func (c *Message) Typ() Opcode {
	return c.opcode
}

func (c *Message) Bytes() []byte {
	return c.dbuf.Bytes()
}
