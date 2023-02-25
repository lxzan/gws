package main

import (
	"github.com/lxzan/gws"
	"net/http"
)

func main() {
	var upgrader = gws.NewUpgrader(
		gws.WithEventHandler(new(WebSocket)),
	)
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Accept(writer, request)
		if err != nil {
			return
		}
		socket.Listen()
	})

	_ = http.ListenAndServe(":3000", nil)
}

type WebSocket struct {
	gws.BuiltinEventHandler
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	socket.WriteMessage(message.Opcode, message.Data.Bytes())
	message.Close()
}
