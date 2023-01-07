package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/lxzan/gws"
	"log"
	"net/http"
	"sync"
	"time"
)

const PingInterval = 15 * time.Second

//go:embed index.html
var html []byte

func main() {
	var ctx = context.Background()
	var handler = NewWebSocket()
	var config = gws.Config{Authenticate: func(r *gws.Request) bool {
		var name = r.URL.Query().Get("name")
		if name == "" {
			return false
		}
		r.SessionStorage.Store("name", name)
		return true
	}}

	_ = http.ListenAndServe(":3000", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/connect":
			socket, err := gws.Accept(ctx, w, r, handler, config)
			if err != nil {
				return
			}
			socket.Listen()
		default:
			w.Write(html)
		}
	}))
}

func NewWebSocket() *WebSocket {
	return &WebSocket{sessions: &sync.Map{}}
}

type WebSocket struct {
	sessions *sync.Map
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	name, _ := socket.SessionStorage.Load("name")
	if v, ok := c.sessions.Load(name); ok {
		v.(*gws.Conn).SetDeadline(time.Now())
	}
	socket.SetDeadline(time.Now().Add(3 * PingInterval))
	c.sessions.Store(name, socket)
	log.Printf("%s connected\n", name)
}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	name, _ := socket.SessionStorage.Load("name")
	c.sessions.Delete(name)
	log.Printf("onerror, name=%s, msg=%s\n", name, err.Error())
}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	name, _ := socket.SessionStorage.Load("name")
	c.sessions.Delete(name)
	log.Printf("onclose, name=%s, code=%d, msg=%s\n", name, code, string(reason))
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

type Input struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	if b := message.Bytes(); len(b) == 4 && string(b) == "ping" {
		socket.WriteMessage(gws.OpcodeText, []byte("pong"))
		socket.SetDeadline(time.Now().Add(3 * PingInterval))
		return
	}

	var input = &Input{}
	json.Unmarshal(message.Bytes(), input)
	if v, ok := c.sessions.Load(input.To); ok {
		v.(*gws.Conn).WriteMessage(gws.OpcodeText, message.Bytes())
	}
}
