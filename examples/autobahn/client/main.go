package main

import (
	"fmt"
	"github.com/lxzan/gws"
)

func main() {
	const count = 517
	//const count = 301
	for i := 1; i <= count; i++ {
		testCase(i)
	}
}

func testCase(id int) {
	var url = fmt.Sprintf("ws://localhost:9001/runCase?case=%d&agent=gws/client", id)
	var handler = &WebSocket{exit: make(chan struct{})}
	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:             url,
		CompressEnabled:  true,
		CheckUtf8Enabled: true,
	})
	if err != nil {
		return
	}
	go socket.Listen()
	<-handler.exit
}

type WebSocket struct {
	exit chan struct{}
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {

}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	c.exit <- struct{}{}
}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	c.exit <- struct{}{}
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	socket.WritePong(payload)
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {

}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	socket.WriteMessage(message.Opcode, message.Bytes())
}
