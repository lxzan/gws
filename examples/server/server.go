package main

import (
	"github.com/lxzan/gws"
	"log"
	"net/http"
)

func main() {
	upgrader := gws.NewUpgrader(new(Websocket), &gws.ServerOption{})

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Accept(writer, request)
		if err != nil {
			log.Printf("Accept: " + err.Error())
			return
		}
		socket.Listen()
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("%+v", err)
	}
}

type Websocket struct {
	gws.BuiltinEventHandler
}

func (w Websocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
