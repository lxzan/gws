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
	//println("step 5")

	//var content = websocket.CloseGoingAway.Bytes()
	//content = append(content, "Hey!"...)
	//socket.Write(websocket.Opcode_CloseConnection, content)
	socket.Close(websocket.CloseGoingAway, []byte("Hey!"))
	return

	//num := atomic.AddInt64(&count, 1)
	//if num == 1 {
	//	t0 = time.Now().UnixMilli()
	//}
	//if num == 100000 {
	//	t1 = time.Now().UnixMilli()
	//	fmt.Printf("%dms\n", t1-t0)
	//}
	//if num%10000 == 0 {
	//	println(num)
	//}
	//return

	body := m.Bytes()
	defer m.Close()

	var key string
	if len(key) <= 10 {
		key = string(body)
	}
	switch key {
	case "test":
		//socket.WritePing()
		//return

		const count = 1000
		for i := 0; i < count; i++ {
			var size = rand.Intn(1024)
			var k = internal.AlphabetNumeric.Generate(size)
			c.sendKeys[string(k)] = 1
			socket.Write(websocket.Opcode_Text, k)
		}
	case "bench":
		const count = 100000
		for i := 0; i < count; i++ {
			var size = rand.Intn(1024)
			var k = internal.AlphabetNumeric.Generate(size)
			socket.Write(websocket.Opcode_Text, k)
		}
	case "verify":
		if len(c.sendKeys) != len(c.recvKeys) {
			panic("count error")
		}
		for k, _ := range c.sendKeys {
			_, ok := c.recvKeys[k]
			if !ok {
				panic(k + " not exist")
			}
		}
		socket.Write(websocket.Opcode_Text, []byte("ok"))
	case "ok":
	case "reset":
		c.sendKeys = make(map[string]uint8)
		c.recvKeys = make(map[string]uint8)
	case "close":
		socket.Close(1001, nil)
	default:
		c.Lock()
		c.recvKeys[key] = 1
		c.Unlock()
	}
}

func (c *WebSocketHandler) OnClose(socket *websocket.Conn, code websocket.Code, reason []byte) {
	println("onclose: ", code)
}

func (c *WebSocketHandler) OnError(socket *websocket.Conn, err error) {
	//println("onerror: " + err.Error())
}

func (c *WebSocketHandler) OnPing(socket *websocket.Conn, m []byte) {
	println("onping")
}

func (c *WebSocketHandler) OnPong(socket *websocket.Conn, m []byte) {
	println("onpong")
}
