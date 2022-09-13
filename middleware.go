package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
)

type Message struct {
	index      int
	compressed bool
	opcode     Opcode
	data       *bytes.Buffer
}

func NewMessage(messageType Opcode, data []byte) *Message {
	return &Message{
		index:      -1,
		compressed: false,
		opcode:     messageType,
		data:       bytes.NewBuffer(data),
	}
}

func (c *Message) MessageType() Opcode {
	return c.opcode
}

func (c *Message) Bytes() []byte {
	return c.data.Bytes()
}

func (c *Message) Close() {
	_pool.Put(c.data)
	return
}

// call next handler function
func (c *Message) Next(socket *Conn) {
	c.index++
	if c.index < len(socket.middlewares) {
		socket.middlewares[c.index](socket, c)
	} else {
		socket.handler.OnMessage(socket, c)
	}
}

// abort the message
func (c *Message) Abort(socket *Conn) {
	panic(internal.PANIC_ABORT)
}

type HandlerFunc func(socket *Conn, msg *Message)
