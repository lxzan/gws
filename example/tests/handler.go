package main

import (
	websocket "github.com/lxzan/gws"
	"github.com/lxzan/gws/internal"
	"math/rand"
	"sync"
)

var count int64 = 0

var t0, t1 int64

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		Mutex:    sync.Mutex{},
		sendKeys: make(map[string]uint8),
		recvKeys: make(map[string]uint8),
	}
}

type WebSocketHandler struct {
	sync.Mutex
	sendKeys map[string]uint8
	recvKeys map[string]uint8
}

func (c *WebSocketHandler) OnRecover(socket *websocket.Conn, exception interface{}) {
}

func (c *WebSocketHandler) OnOpen(socket *websocket.Conn) {

}

var once = sync.Once{}

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
		const count = 100000
		for i := 0; i < count; i++ {
			var size = rand.Intn(1024)
			var k = internal.AlphabetNumeric.Generate(size)
			socket.Write(websocket.Opcode_Text, k)
		}
	case "verify":
		c.OnVerify(socket)
	case "ok":
	case "reset":
		c.sendKeys = make(map[string]uint8)
		c.recvKeys = make(map[string]uint8)
	case "close":
		socket.Close(1001, nil)
	default:
		c.Lock()
		socket.Storage.Delete(key)
		c.Unlock()
	}
}

func (c *WebSocketHandler) OnClose(socket *websocket.Conn, code websocket.Code, reason []byte) {
	println("onclose: ", code)
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
