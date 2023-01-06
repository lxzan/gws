# gws
### minimal websocket server

#### Highlight
- websocket event api
- zero dependency
- zero extra goroutine to control websocket
- zero error to read/write message, errors have been handled appropriately
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
    OnClose(socket *Conn, code uint16, reason []byte)
    OnPing(socket *Conn, payload []byte)
    OnPong(socket *Conn, payload []byte)
    OnMessage(socket *Conn, message *Message)
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
	var config = gws.Config{CompressEnabled: true, CheckTextEncoding: true, MaxContentLength: 32 * 1024 * 1024}
	var handler = new(WebSocket)

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := gws.Accept(context.Background(), writer, request, handler, config)
		if err != nil {
			return
		}
		socket.Listen()
	})

	_ = http.ListenAndServe(":3000", nil)
}

type WebSocket struct{}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	fmt.Printf("onclose: code=%d, payload=%s\n", code, string(reason))
}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	fmt.Printf("onerror: err=%s\n", err.Error())
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	println("connected")
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	fmt.Printf("onping: payload=%s\n", string(payload))
	socket.WritePong(payload)
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	socket.WriteMessage(message.Typ(), message.Bytes())
	message.Close()
}
```

#### HeartBeat
```go
const PingInterval = 5*time.Second

type WebSocket struct {}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	socket.SetDeadline(time.Now().Add(3*PingInterval))
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	socket.WritePong(nil)
	socket.SetDeadline(time.Now().Add(3*PingInterval))
}
```

#### Test
```bash
// Terminal 1
git clone https://github.com/lxzan/gws.git 
cd gws
go run github.com/lxzan/gws/examples/testsuite

// Terminal 2
cd examples/testsuite
docker run -it --rm \
    -v ${PWD}/config:/config \
    -v ${PWD}/reports:/reports \
    crossbario/autobahn-testsuite \
    wstest -m fuzzingclient -s /config/fuzzingclient.json
```
