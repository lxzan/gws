# gws
### minimal websocket server

#### Highlight
- websocket event api
- write in batch and flush 
- no dependency
- zero goroutine to control websocket
- fully passes the WebSocket [autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite)

#### Attention
- It's designed for api server, do not write big message
- It's recommended not to enable data compression in the intranet
- WebSocket Events are emitted synchronously, manage goroutines yourself

#### Interface
```go
type Event interface {
	OnOpen(socket *Conn)
	OnError(socket *Conn, err error)
	OnClose(socket *Conn, message *Message)
	OnMessage(socket *Conn, message *Message)
	OnPing(socket *Conn, message *Message)
	OnPong(socket *Conn, message *Message)
}
```

#### Quick Start
```go
package main

import (
	"context"
	"fmt"
	"github.com/lxzan/gws"
	"net/http"
)

func main() {
	var upgrader = gws.Upgrader{CompressEnabled: true, MaxContentLength: 32 * 1024 * 1024}
	var handler = new(WebSocket)

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(context.Background(), writer, request, handler)
		if err != nil {
			return
		}

		defer socket.Close()
		socket.Listen()
	})

	_ = http.ListenAndServe(":3000", nil)
}

type WebSocket struct{}

func (c *WebSocket) OnClose(socket *gws.Conn, message *gws.Message) {
	fmt.Printf("onclose: code=%d, payload=%s\n", message.Code(), string(message.Bytes()))
	message.Close()
}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	fmt.Printf("onerror: err=%s\n", err.Error())
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	println("connected")
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	socket.WriteMessage(message.Typ(), message.Bytes())
	message.Close()
}

func (c *WebSocket) OnPing(socket *gws.Conn, message *gws.Message) {
	fmt.Printf("onping: payload=%s\n", string(message.Bytes()))
	socket.WritePong(message.Bytes())
	message.Close()
}

func (c *WebSocket) OnPong(socket *gws.Conn, message *gws.Message) {}
```
