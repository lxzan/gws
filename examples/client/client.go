package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/lxzan/gws"
)

func main() {
	socket, _, err := gws.NewClient(new(WebSocket), &gws.ClientOption{
		Addr: "ws://127.0.0.1:3000/connect",
	})
	if err != nil {
		log.Printf(err.Error())
		return
	}
	go socket.ReadLoop()

	for {
		var text = ""
		fmt.Scanf("%s", &text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		socket.WriteString(text)
	}
}

type WebSocket struct {
}

func (c *WebSocket) OnClose(socket *gws.Conn, err error) {
	fmt.Printf("onerror: err=%s\n", err.Error())
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	_ = socket.WriteString("hello, there is client")
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	fmt.Printf("recv: %s\n", message.Data.String())
}
