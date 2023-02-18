# gws

### event-driven go websocket server

[![Build Status][1]][2] [![MIT licensed][3]][4] [![Go Version][5]][6] [![codecov][7]][8] [![Go Report Card][9]][10]

[1]: https://github.com/lxzan/gws/workflows/Go%20Test/badge.svg?branch=master

[2]: https://github.com/lxzan/gws/actions?query=branch%3Amaster

[3]: https://img.shields.io/badge/license-MIT-blue.svg

[4]: LICENSE

[5]: https://img.shields.io/badge/go-%3E%3D1.16-30dff3?style=flat-square&logo=go

[6]: https://github.com/lxzan/gws

[7]: https://codecov.io/github/lxzan/gws/branch/master/graph/badge.svg?token=DJU7YXWN05

[8]: https://app.codecov.io/gh/lxzan/gws

[9]: https://goreportcard.com/badge/github.com/lxzan/gws

[10]: https://goreportcard.com/report/github.com/lxzan/gws


- [gws](#gws)
  - [Highlight](#highlight)
  - [Attention](#attention)
  - [Benchmark](#benchmark)
  - [Core Interface](#core-interface)
  - [Install](#install)
  - [Examples](#examples)
  - [Quick Start (Autobahn Server)](#quick-start-autobahn-server)
  - [TLS](#tls)
  - [Autobahn Test](#autobahn-test)

#### Highlight

- zero dependency, not use channel but event driven
- zero extra goroutine to manage connection
- zero error to read/write operation, errors have been handled appropriately
- built-in concurrent_map implementation
- fully passes the WebSocket [autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite)

#### Attention

- It's designed for api server, do not write big message.
- WebSocket events are emitted synchronously, manage goroutines yourself.
- Write operations are synchronous blocking, for broadcast scenarios, special attention is needed.

#### Benchmark

- machine: Ubuntu 20.04LTS VM (4C8T)
- client: tcpkali
- payload: 2.34KiB

| Server  | Connection | Send Speed (msg/s) | T (s) | Download / Upload Bandwidth (Mbps) |
| ------- | ---------- | ------------------ | ----- | ---------------------------------- |
| gws     | 200        | 4000               | 30    | 10916.094↓ 10951.587↑              |
| gorilla | 200        | 4000               | 30    | 4344.222↓ 4380.711↑                |
| gws     | 2000       | 300                | 30    | 7941.090↓ 7951.117↑                |
| gorilla | 2000       | 300                | 30    | 4706.938↓ 4715.744↑                |
| gws     | 5000       | 60                 | 30    | 5891.151↓ 5908.599↑                |
| gorilla | 5000       | 60                 | 30    | -                                  |
| gws     | 10000      | 10                 | 60    | 1980.124↓ 1977.561↑                |
| gorilla | 10000      | 10                 | 60    | 1972.556↓ 1979.981↑                |
| gws     | 10000      | 20                 | 60    | 3952.788↓ 3959.341↑                |
| gorilla | 10000      | 20                 | 60    | -                                  |

> `-` means exception

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

#### Examples
- [chat room](examples/chatroom/main.go)
- [echo](examples/testsuite/main.go)

#### Quick Start (Autobahn Server)

```go
package main

import (
	"fmt"
	"github.com/lxzan/gws"
	"net/http"
)

func main() {
	var upgrader = gws.NewUpgrader(func(c *gws.Upgrader) {
		c.CompressEnabled = true
		c.CheckTextEncoding = true
		c.MaxContentLength = 32 * 1024 * 1024
		c.EventHandler = new(WebSocket)
	})

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Accept(writer, request)
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
	upgrader := gws.NewUpgrader(gws.WithEventHandler(handler))
	app.GET("/connect", func(ctx *gin.Context) {
		socket, err := upgrader.Accept(ctx.Writer, ctx.Request)
		if err != nil {
			return
		}
		upgrader.Listen(socket)
	})
	cert := "server.crt"
	key := "server.key"
	if err := app.RunTLS(":8443", cert, key); err != nil {
		panic(err)
	}
}
```

#### Autobahn Test

```bash
cd examples/testsuite
mkdir reports
docker run -it --rm \
    -v ${PWD}/config:/config \
    -v ${PWD}/reports:/reports \
    crossbario/autobahn-testsuite \
    wstest -m fuzzingclient -s /config/fuzzingclient.json
```
