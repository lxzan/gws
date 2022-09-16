package main

import (
	"github.com/lxzan/gws"
	"github.com/lxzan/gws/internal"
	"math/rand"
	"time"
)

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{}
}

type WebSocketHandler struct{}

func (c *WebSocketHandler) OnRecover(socket *gws.Conn, exception interface{}) {

}

func (c *WebSocketHandler) OnOpen(socket *gws.Conn) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-socket.Context().Done():
				println("connection closed")
				return
			case <-ticker.C:
				socket.WritePing(nil)
			}
		}
	}()
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
		socket.WritePing(nil)
	case "pong":
		socket.WritePong(nil)
	case "close":
		socket.Close(1001, []byte("goodbye"))
	default:
		socket.Storage.Delete(key)
	}
}

func (c *WebSocketHandler) OnClose(socket *gws.Conn, code gws.Code, reason []byte) {
}

func (c *WebSocketHandler) OnError(socket *gws.Conn, err error) {
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
		var size = internal.AlphabetNumeric.Intn(1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.Storage.Put(string(k), 1)
		socket.Write(gws.OpcodeText, k)
	}
}

func (c *WebSocketHandler) OnVerify(socket *gws.Conn) {
	if socket.Storage.Len() != 0 {
		panic("failed")
	}

	socket.Write(gws.OpcodeText, []byte("ok"))
}

func (c *WebSocketHandler) OnBench(socket *gws.Conn) {
	const count = 1000000
	for i := 0; i < count; i++ {
		var size = 10 + rand.Intn(1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.Write(gws.OpcodeText, k)
	}
}
