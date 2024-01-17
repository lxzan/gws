package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lxzan/gws"
)

const (
	agent      = "gws/client@v1.8.0"
	remoteAddr = "127.0.0.1:9001"
)

func main() {
	const count = 517
	for i := 1; i <= count; i++ {
		testCase(i)
	}
	updateReports()
}

func testCase(id int) {
	var url = fmt.Sprintf("ws://%s/runCase?case=%d&agent=%s", remoteAddr, id, agent)
	var handler = &WebSocket{onexit: make(chan struct{})}
	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:                url,
		ReadAsyncEnabled:    true,
		CheckUtf8Enabled:    true,
		ReadMaxPayloadSize:  32 * 1024 * 1024,
		WriteMaxPayloadSize: 32 * 1024 * 1024,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})
	if err != nil {
		log.Println(err.Error())
		return
	}
	go socket.ReadLoop()
	<-handler.onexit
}

type WebSocket struct {
	onexit chan struct{}
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(30 * time.Second))
}

func (c *WebSocket) OnClose(socket *gws.Conn, err error) {
	c.onexit <- struct{}{}
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	socket.WriteAsync(message.Opcode, message.Bytes(), nil)
}

type updateReportsHandler struct {
	onexit chan struct{}
	gws.BuiltinEventHandler
}

func (c *updateReportsHandler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(5 * time.Second))
}

func (c *updateReportsHandler) OnClose(socket *gws.Conn, err error) {
	c.onexit <- struct{}{}
}

func updateReports() {
	var url = fmt.Sprintf("ws://%s/updateReports?agent=gws/client", remoteAddr)
	var handler = &updateReportsHandler{onexit: make(chan struct{})}
	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:             url,
		CheckUtf8Enabled: true,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})
	if err != nil {
		log.Println(err.Error())
		return
	}
	go socket.ReadLoop()
	<-handler.onexit
}
