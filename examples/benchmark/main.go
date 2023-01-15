package main

import (
	"github.com/lxzan/gws"
	"net/http"
)

func main() {
	var handler = new(WebSocket)
	var upgrader = gws.NewUpgrader(gws.WithEventHandler(handler))
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Accept(writer, request)
		if err != nil {
			return
		}
		socket.Listen()
	})

	_ = http.ListenAndServe(":3000", nil)
}

type WebSocket struct{}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	socket.WriteMessage(message.Typ(), message.Bytes())
	message.Close()
}
