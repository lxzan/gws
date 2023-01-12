# gws

### event-driven websocket server

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
- client: tcpkali


| Server  | Connection | Send Speed(msg/s) | Payload size | Download Bandwidth(Mbps) | Upload Bandwidth(Mbps) |
| ------- | ---------- | ----------------- | ------------ | ------------------------ | ---------------------- |
| gws     | 200        | 20000             | 2.34KiB      | 12080.082↓               | 12101.773↑             |
| gorilla | 200        | 20000             | 2.34KiB      | 6285.944↓                | 6308.428↑              |
| gws     | 2000       | 100               | 2.34KiB      | 3950.930↓                | 3957.692↑              |
| gorilla | 2000       | 100               | 2.34KiB      | 3953.384↓                | 3960.681↑              |
| gws     | 5000       | 40                | 2.34KiB      | 2534.289↓                | 2559.629↑              |
| gorilla | 5000       | 40                | 2.34KiB      | -                        | -                      |
| gws     | 10000      | 4                 | 2.34KiB      | 789.898↓                 | 791.224↑               |
| gorilla | 10000      | 4                 | 2.34KiB      | 785.721↓                 | 787.274↑               |
| gws     | 10000      | 8                 | 2.34KiB      | 1578.890↓                | 1581.459↑              |
| gorilla | 10000      | 8                 | 2.34KiB      | -                        | -                      |
> `-` means exception
