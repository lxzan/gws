# gws
## a event driven websocket framework

### Features
- event-driven api design
- use queues to process messages parallelly, with concurrency control
- middleware support
- self-implementing buffer, more memory-saving
- no dependency

### Quick Start
chat room example:

server
```go
package main

import (
	"context"
	"encoding/json"
	"github.com/lxzan/gws"
	"net/http"
	"sync"
)

type Handler struct {
	sessions sync.Map
}

func (h *Handler) OnOpen(s *gws.Conn) {
	name, _ := s.Storage.Get("name")
	h.sessions.Store(name.(string), s)

	go func(socket *gws.Conn, key string) {
		for {
			<-socket.Context.Done()
			h.sessions.Delete(key)
		}
	}(s, name.(string))
}

func (h *Handler) OnClose(socket *gws.Conn, code gws.Code, reason []byte) {}

func (h *Handler) OnMessage(socket *gws.Conn, m *gws.Message) {
	var request = struct {
		To      string `json:"to"`
		Message string `json:"message"`
	}{}
	json.Unmarshal(m.Bytes(), &request)
	defer m.Close()

	if receiver, ok := h.sessions.Load(request.To); ok {
		receiver.(*gws.Conn).Write(gws.OpcodeText, m.Bytes())
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
			if name := r.URL.Query().Get("name"); name != "" {
				r.Storage.Put("name", name)
				return true
			}
			return false
		},
	}

	var handler = &Handler{sessions: sync.Map{}}
	var ctx = context.Background()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		upgrader.Upgrade(ctx, w, r, handler)
	})

	http.ListenAndServe(":3000", nil)
}
```

client(browser console)
```js
let ws1 = new WebSocket('ws://127.0.0.1:3000/ws?name=caster');
let ws2 = new WebSocket('ws://127.0.0.1:3000/ws?name=lancer');
ws1.send('{"to": "lancer", "msg": "Hello! I am caster"}');
```

### API

```go
type EventHandler interface {
    OnOpen(socket *Conn)
    OnClose(socket *Conn, code Code, reason []byte)
    OnMessage(socket *Conn, m *Message)
    OnError(socket *Conn, err error)
    OnPing(socket *Conn, m []byte)
    OnPong(socket *Conn, m []byte)
}
```

### Usage

#### Middleware

- use internal middleware

```go
var upgrader = gws.Upgrader{}
upgrader.Use(gws.Recovery(func(exception interface{}) {
    fmt.Printf("%v", exception)
}))
```

- write a middleware

```go
upgrader.Use(func (socket *gws.Conn, msg *gws.Message) {
    var t0 = time.Now().UnixNano()
    msg.Next(socket)
    var t1 = time.Now().UnixNano()
    fmt.Printf("cost=%dms\n", (t1-t0)/1000000)
})
```

#### Heartbeat

- Sever Side Heartbeat

```go
func (h *Handler) OnOpen(socket *gws.Conn) {
    go func (ws *gws.Conn) {
        ticker := time.NewTicker(15 * time.Second)
        defer ticker.Stop()
    
        for {
            select {
            case <-ticker.C:
            	ws.WritePing(nil)
            case <-ws.Context.Done():
            	return
            }
        }

    }(socket)
}

func (h *Handler) OnPong(socket *gws.Conn, m []byte) {
    _ = socket.SetDeadline(30 * time.Second)
}
```

- Client Side Heartbeat
```go
func (h *Handler) OnPing(socket *gws.Conn, m []byte) {
    socket.WritePong(nil)
    _ = socket.SetDeadline(30 * time.Second)
}
```
