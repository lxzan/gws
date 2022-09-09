package websocket

import "bytes"

const PANIC_SIGNAL_ABORT = "PANIC_SIGNAL_ABORT"

type Message struct {
	index      int
	compressed bool
	opcode     Opcode
	data       *bytes.Buffer
}

func (c *Message) MessageType() Opcode {
	return c.opcode
}

func (c *Message) Bytes() []byte {
	return c.data.Bytes()
}

func (c *Message) Close() error {
	_pool.Put(c.data)
	return nil
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
	panic(PANIC_SIGNAL_ABORT)
}

type HandlerFunc func(socket *Conn, msg *Message)
