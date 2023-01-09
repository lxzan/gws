# gws

### event-driven websocket server

[![Build Status][1]][2] [![MIT licensed][3]][4] [![Go Version][5]][6]

[1]: https://github.com/lxzan/gws/workflows/Go%20Test/badge.svg?branch=master

[2]: https://github.com/lxzan/gws/actions?query=branch%3Amaster

[3]: https://img.shields.io/badge/license-MIT-blue.svg

[4]: LICENSE

[5]: https://img.shields.io/badge/go-%3E%3D1.16-30dff3?style=flat-square&logo=go

[6]: https://github.com/lxzan/gws

#### Highlight

- zero dependency, no channel but event driven
- zero extra goroutine to manage connection
- zero error to read/write operation, errors have been handled appropriately
- built-in concurrent_map implementation
- fully passes the WebSocket [autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite)

#### Attention

- It's designed for api server, do not write big message
- It's recommended not to enable data compression in the intranet
- WebSocket Events are emitted synchronously, manage goroutines yourself

#### Core Interface

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

#### Install

```bash
go get -v github.com/lxzan/gws@latest
```

#### Quick Start

```go
package main

import (
	"fmt"
	"github.com/lxzan/gws"
	"net/http"
)

func main() {
	var config = &gws.Config{
		CompressEnabled:   true,
		CheckTextEncoding: true,
		MaxContentLength:  32 * 1024 * 1024,
	}
	var handler = new(WebSocket)
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := gws.Accept(writer, request, handler, config)
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

#### TLS

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/lxzan/gws"
)

func main() {
	app := gin.New()
	handler := new(WebSocket)
	app.GET("/connect", func(ctx *gin.Context) {
		socket, err := gws.Accept(ctx.Writer, ctx.Request, handler, nil)
		if err != nil {
			return
		}
		socket.Listen()
	})
	cert := "server.crt"
	key := "server.key"
	if err := app.RunTLS(":8443", cert, key); err != nil {
		panic(err)
	}
}
```

#### Test

```bash
cd examples/testsuite
mkdir reports
docker run -it --rm \
    -v ${PWD}/config:/config \
    -v ${PWD}/reports:/reports \
    crossbario/autobahn-testsuite \
    wstest -m fuzzingclient -s /config/fuzzingclient.json
```

#### Benchmark

- machine: MacBook Pro M1
- body.json size: 2.34KiB
- max cost: cpu=440% memory=90MiB
- command: `tcpkali -c 200 -r 20000 -T 20s -f body.json --ws 127.0.0.1:3000/connect`

##### lxzan/gws

```
Destination: [127.0.0.1]:3000
Interface lo0 address [127.0.0.1]:0
Using interface lo0 to connect to [127.0.0.1]:3000
Ramped up to 200 connections.
Total data sent:     28864.1 MiB (30266158036 bytes)
Total data received: 28812.3 MiB (30211911226 bytes)
Bandwidth per channel: 120.909⇅ Mbps (15113.7 kBps)
Aggregate bandwidth: 12080.082↓, 12101.773↑ Mbps
Packet rate estimate: 1098496.2↓, 1105141.9↑ (8↓, 14↑ TCP MSS/op)
Test duration: 20.0078 s.
```

##### gorilla/websocket

```
Destination: [127.0.0.1]:3000
Interface lo0 address [127.0.0.1]:0
Using interface lo0 to connect to [127.0.0.1]:3000
Ramped up to 200 connections.
Total data sent:     15043.6 MiB (15774402996 bytes)
Total data received: 14990.0 MiB (15718181647 bytes)
Bandwidth per channel: 62.972⇅ Mbps (7871.5 kBps)
Aggregate bandwidth: 6285.944↓, 6308.428↑ Mbps
Packet rate estimate: 589201.4↓, 579598.1↑ (5↓, 12↑ TCP MSS/op)
Test duration: 20.0042 s.
```
