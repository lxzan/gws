package gws

import (
	"github.com/lxzan/gws/internal"
)

type Message struct {
	index      int
	abort      bool
	compressed bool
	opcode     Opcode
	data       *internal.Buffer
}

func NewMessage(compressed bool, messageType Opcode, data *internal.Buffer) *Message {
	return &Message{
		index:      0,
		abort:      false,
		compressed: compressed,
		opcode:     messageType,
		data:       data,
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
	if c.abort {
		return
	}

	if c.index < len(socket.middlewares) {
		c.index++
		socket.middlewares[c.index-1](socket, c)
	} else {
		socket.handler.OnMessage(socket, c)
	}
}

// abort the next handlerFuncs, but previous handlerFuncs will be executed
func (c *Message) Abort(socket *Conn) {
	c.abort = true
}

type HandlerFunc func(socket *Conn, msg *Message)
