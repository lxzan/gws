package main

import (
	"github.com/lxzan/gws"
	"github.com/lxzan/gws/internal"
	"math/rand"
)

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{}
}

type WebSocketHandler struct{}

func (c *WebSocketHandler) OnOpen(socket *gws.Conn) {

}

func (c *WebSocketHandler) OnMessage(socket *gws.Conn, m *gws.Message) {
	body := m.Bytes()
	defer m.Close()

	var key string
	if len(key) <= 10 {
		key = string(body)
	}
	switch key {
	case "test":
		c.OnTest(socket)
	case "bench":
		c.OnBench(socket)
	case "verify":
		c.OnVerify(socket)
	case "ok":
	case "ping":
		socket.WriteMessage(gws.OpcodePing, nil)
	case "pong":
		socket.WriteMessage(gws.OpcodePong, nil)
	case "close":
		socket.WriteClose(gws.CloseGoingAway, []byte("goodbye"))
		socket.Close()
	default:
		socket.Storage.Delete(key)
	}
}

func (c *WebSocketHandler) OnClose(socket *gws.Conn, code gws.CloseCode, reason []byte) {
}

func (c *WebSocketHandler) OnError(socket *gws.Conn, err error) {
	println(err.Error())
}

func (c *WebSocketHandler) OnPing(socket *gws.Conn, m []byte) {
	println("onping")
}

func (c *WebSocketHandler) OnPong(socket *gws.Conn, m []byte) {
	println("onpong")
}

func (c *WebSocketHandler) OnTest(socket *gws.Conn) {
	const count = 1000
	for i := 0; i < count; i++ {
		var size = internal.AlphabetNumeric.Intn(8 * 1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.Storage.Put(string(k), 1)
		socket.WriteMessage(gws.OpcodeText, k)
	}
}

func (c *WebSocketHandler) OnVerify(socket *gws.Conn) {
	if socket.Storage.Len() != 0 {
		socket.WriteMessage(gws.OpcodeText, []byte("failed"))
	}

	socket.WriteMessage(gws.OpcodeText, []byte("ok"))
}

func (c *WebSocketHandler) OnBench(socket *gws.Conn) {
	const count = 1000000
	for i := 0; i < count; i++ {
		var size = 10 + rand.Intn(1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.WriteMessage(gws.OpcodeText, k)
		//socket.WriteMessage(gws.OpcodeText, []byte("Hello"))
	}
}
