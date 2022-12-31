package gws

import (
	"github.com/lxzan/gws/internal"
)

type Message struct {
	err    error
	opcode Opcode
	dbuf   *internal.Buffer // 数据缓冲
	cbuf   *internal.Buffer // 解码器缓冲
}

//func NewMessage(compressed bool, messageType Opcode, data *internal.Buffer) *Message {
//	return &Message{
//		compressed: compressed,
//		opcode:     messageType,
//		dbuf:       data,
//	}
//}

func (c *Message) Err() error {
	return c.err
}

func (c *Message) Typ() Opcode {
	return c.opcode
}

func (c *Message) Bytes() []byte {
	return c.dbuf.Bytes()
}

func (c *Message) Close() {
	_pool.Put(c.dbuf)
	if c.cbuf != nil {
		_pool.Put(c.cbuf)
	}
	return
}
