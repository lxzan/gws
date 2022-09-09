package main

import (
	websocket "github.com/lxzan/gws"
	"github.com/lxzan/gws/internal"
	"math/rand"
)

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{}
}

type WebSocketHandler struct{}

func (c *WebSocketHandler) OnRecover(socket *websocket.Conn, exception interface{}) {
}

func (c *WebSocketHandler) OnOpen(socket *websocket.Conn) {

}

func (c *WebSocketHandler) OnMessage(socket *websocket.Conn, m *websocket.Message) {
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
		socket.WritePing(nil)
	case "pong":
		socket.WritePong(nil)
	case "close":
		socket.Close(1001, []byte("goodbye"))
	default:
		socket.Storage.Delete(key)
	}
}

func (c *WebSocketHandler) OnClose(socket *websocket.Conn, code websocket.Code, reason []byte) {
}

func (c *WebSocketHandler) OnError(socket *websocket.Conn, err error) {
}

func (c *WebSocketHandler) OnPing(socket *websocket.Conn, m []byte) {
	println("onping")
}

func (c *WebSocketHandler) OnPong(socket *websocket.Conn, m []byte) {
	println("onpong")
}

func (c *WebSocketHandler) OnTest(socket *websocket.Conn) {
	const count = 1000
	for i := 0; i < count; i++ {
		var size = rand.Intn(1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.Storage.Put(string(k), 1)
		socket.Write(websocket.Opcode_Text, k)
	}
}

func (c *WebSocketHandler) OnVerify(socket *websocket.Conn) {
	if socket.Storage.Len() != 0 {
		panic("failed")
	}

	socket.Write(websocket.Opcode_Text, []byte("ok"))
}

func (c *WebSocketHandler) OnBench(socket *websocket.Conn) {
	const count = 100000
	for i := 0; i < count; i++ {
		var size = rand.Intn(1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.Write(websocket.Opcode_Text, k)
	}
}
