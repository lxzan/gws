package main

import (
	"log"

	"github.com/lxzan/gws"
)

func main() {
	var app = gws.NewServer(new(Handler), &gws.ServerOption{
		CompressEnabled:  true,
		CheckUtf8Enabled: true,
	})
	log.Fatalf("%v", app.Run(":8000"))
}

type Handler struct {
	gws.BuiltinEventHandler
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	socket.PushTask(func() {
		_ = socket.WriteMessage(message.Opcode, message.Bytes())
		_ = message.Close()
	})
}
