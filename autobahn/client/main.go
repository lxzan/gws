package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lxzan/gws"
)

const remoteAddr = "localhost:9001"

func main() {
	const count = 517
	for i := 1; i <= count; i++ {
		testCase(i)
	}
	updateReports()
}

func testCase(id int) {
	var url = fmt.Sprintf("ws://%s/runCase?case=%d&agent=gws/client", remoteAddr, id)
	var handler = &WebSocket{onexit: make(chan struct{})}
	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:                url,
		ReadAsyncEnabled:    true,
		CompressEnabled:     true,
		CheckUtf8Enabled:    true,
		ReadMaxPayloadSize:  32 * 1024 * 1024,
		WriteMaxPayloadSize: 32 * 1024 * 1024,
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

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	c.onexit <- struct{}{}
}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	c.onexit <- struct{}{}
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	_ = socket.WriteAsync(message.Opcode, message.Bytes())
}

type updateReportsHandler struct {
	onexit chan struct{}
	gws.BuiltinEventHandler
}

func (c *updateReportsHandler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(5 * time.Second))
}

func (c *updateReportsHandler) OnError(socket *gws.Conn, err error) {
	c.onexit <- struct{}{}
}

func (c *updateReportsHandler) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	c.onexit <- struct{}{}
}

func updateReports() {
	var url = fmt.Sprintf("ws://%s/updateReports?agent=gws/client", remoteAddr)
	var handler = &updateReportsHandler{onexit: make(chan struct{})}
	socket, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr:             url,
		CompressEnabled:  true,
		CheckUtf8Enabled: true,
	})
	if err != nil {
		log.Println(err.Error())
		return
	}
	go socket.ReadLoop()
	<-handler.onexit
}
