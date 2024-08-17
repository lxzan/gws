package main

import (
	"github.com/lxzan/gws"
	"log"
	"net/http"
	"os"
)

func main() {
	upgrader := gws.NewUpgrader(&Handler{}, &gws.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			return
		}
		go func() {
			socket.ReadLoop()
		}()
	})
	log.Panic(
		http.ListenAndServe(":8000", nil),
	)
}

type Handler struct {
	gws.BuiltinEventHandler
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	//file, _ := os.OpenFile("C:\\msys64\\home\\lxzan\\Open\\gws\\assets\\github.json", os.O_RDONLY, 0644)
	file, _ := os.OpenFile("C:\\Users\\lxzan\\Pictures\\mg.png", os.O_RDONLY, 0644)
	defer file.Close()
	_ = socket.WriteReader(gws.OpcodeBinary, file)
	//_ = socket.WriteReader(message.Opcode, message)
}
