package main

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/lxzan/gws"
	"github.com/lxzan/gws/internal"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"time"
)

//go:embed index.html
var html []byte

func main() {
	var handler = NewWebSocket()
	var ctx = context.Background()

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := gws.Accept(ctx, writer, request, handler, gws.Config{})
		if err != nil {
			return
		}
		socket.Listen()
	})

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		writer.Write(html)
	})

	http.ListenAndServe(":3000", nil)
}

func NewWebSocket() *WebSocket {
	return &WebSocket{}
}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	fmt.Printf("onclose: code=%d, payload=%s\n", code, string(reason))
}

type WebSocket struct{}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	println("connected")
}

func (c *WebSocket) OnMessage(socket *gws.Conn, m *gws.Message) {
	defer m.Close()

	var key = string(m.Bytes())
	switch key {
	case "test":
		c.OnTest(socket)
	case "bench":
		c.OnBench(socket)
	case "verify":
		c.OnVerify(socket)
	case "ok":
	case "ping":
		socket.WriteMessage(gws.OpcodePing, nil)
	case "pong":
		socket.WriteMessage(gws.OpcodePong, nil)
	case "close":
		socket.WriteClose(1001, []byte("goodbye"))
	default:
		socket.SessionStorage.Delete(key)
	}
}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	println(err.Error())
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	socket.WritePong(nil)
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {
	println("onpong")
}

func (c *WebSocket) OnTest(socket *gws.Conn) {
	const count = 1000
	for i := 0; i < count; i++ {
		var size = internal.AlphabetNumeric.Intn(8 * 1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.SessionStorage.Store(string(k), 1)
		socket.WriteMessage(gws.OpcodeText, k)
	}
}

func (c *WebSocket) OnVerify(socket *gws.Conn) {
	var length = 0
	socket.SessionStorage.Range(func(key, value interface{}) bool {
		length++
		return false
	})
	if length != 0 {
		socket.WriteMessage(gws.OpcodeText, []byte("failed"))
		return
	}
	socket.WriteMessage(gws.OpcodeText, []byte("ok"))
}

func (c *WebSocket) OnBench(socket *gws.Conn) {
	var t0 = time.Now()
	const count = 1000000
	for i := 0; i < count; i++ {
		var size = 10 + rand.Intn(1024)
		var k = internal.AlphabetNumeric.Generate(size)
		socket.WriteMessage(gws.OpcodeText, k)
		//socket.WriteMessage(gws.OpcodeText, []byte("Hello"))
	}
	println(time.Since(t0).String())
}
