package main

import (
	"fmt"
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
}

func (w Websocket) OnOpen(socket *gws.Conn) {
	_ = socket.WriteString("hello, there is server")
}

func (w Websocket) OnError(socket *gws.Conn, err error) {
	fmt.Printf("onerror: err=%s\n", err.Error())
}

func (w Websocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	fmt.Printf("onclose: code=%d, payload=%s\n", code, string(reason))
}

func (w Websocket) OnPing(socket *gws.Conn, payload []byte) {
}

func (w Websocket) OnPong(socket *gws.Conn, payload []byte) {
	socket.WritePong(payload)
}

func (w Websocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	fmt.Printf("recv: %s\n", message.Data.String())
}
