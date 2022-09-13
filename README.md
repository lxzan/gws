# gws
#### event driven websocket framework

### Quick Start
chat room
```go
package main

import (
	"encoding/json"
	"github.com/lxzan/gws"
	"net/http"
	"sync"
)

var handler = &Handler{sessions: sync.Map{}}

type Handler struct {
	sessions sync.Map
}

func (h *Handler) OnRecover(socket *gws.Conn, exception interface{}) {}

func (h *Handler) OnOpen(socket *gws.Conn) {
	name, _ := socket.Storage.Get("name")
	h.sessions.Store(name.(string), socket)
}

func (h *Handler) OnClose(socket *gws.Conn, code gws.Code, reason []byte) {}

type Request struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

func (h *Handler) OnMessage(socket *gws.Conn, m *gws.Message) {
	var request Request
	json.Unmarshal(m.Bytes(), &request)

	me, _ := socket.Storage.Get("name")
	if me.(string) == request.To {
		socket.Write(m.MessageType(), m.Bytes())
		m.Close()
	} else {
		if receiver, ok := h.sessions.Load(request.To); ok {
			h.OnMessage(receiver.(*gws.Conn), m)
		}
	}
}

func (h *Handler) OnError(socket *gws.Conn, err error) {}

func (h *Handler) OnPing(socket *gws.Conn, m []byte) {}

func (h *Handler) OnPong(socket *gws.Conn, m []byte) {}

func main() {
	var upgrader = gws.Upgrader{
		ServerOptions: &gws.ServerOptions{
			LogEnabled:      true,
			CompressEnabled: false,
		},
		CheckOrigin: func(r *gws.Request) bool {
			r.Storage.Put("name", r.URL.Query().Get("name"))
			return true
		},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		upgrader.Upgrade(w, r, handler)
	})

	http.ListenAndServe(":3000", nil)
}
```

### Core
```go
type EventHandler interface {
	OnRecover(socket *Conn, exception interface{})
	OnOpen(socket *Conn)
	OnClose(socket *Conn, code Code, reason []byte)
	OnMessage(socket *Conn, m *Message)
	OnError(socket *Conn, err error)
	OnPing(socket *Conn, m []byte)
	OnPong(socket *Conn, m []byte)
}
```

### Usage
- Middleware
```go
var upgrader = gws.Upgrader{}

upgrader.Use(func(socket *gws.Conn, msg *gws.Message) {
    var t0 = time.Now().UnixNano()
    msg.Next(socket)
    var t1 = time.Now().UnixNano()
    fmt.Printf("cost=%dms\n", (t1-t0)/1000000)
})
```

- Sever Side HeartBeat
```go
func (h *Handler) OnOpen(socket *gws.Conn) {
	go func(ws *gws.Conn) {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ws.WritePing(nil)
			case <-ws.Context().Done():
				return
			}
		}

	}(socket)
}

func (h *Handler) OnPong(socket *gws.Conn, m []byte) {
	_ = socket.SetDeadline(30 * time.Second)
}
```