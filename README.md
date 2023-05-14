# gws

### event-driven go websocket server & client

[![Build Status][1]][2] [![MIT licensed][3]][4] [![Go Version][5]][6] [![codecov][7]][8] [![Go Report Card][9]][10]

[1]: https://github.com/lxzan/gws/workflows/Go%20Test/badge.svg?branch=master

[2]: https://github.com/lxzan/gws/actions?query=branch%3Amaster

[3]: https://img.shields.io/badge/license-MIT-blue.svg

[4]: LICENSE

[5]: https://img.shields.io/badge/go-%3E%3D1.18-30dff3?style=flat-square&logo=go

[6]: https://github.com/lxzan/gws

[7]: https://codecov.io/github/lxzan/gws/branch/master/graph/badge.svg?token=DJU7YXWN05

[8]: https://app.codecov.io/gh/lxzan/gws

[9]: https://goreportcard.com/badge/github.com/lxzan/gws

[10]: https://goreportcard.com/report/github.com/lxzan/gws

- [gws](#gws)
	- [Highlight](#highlight)
	- [Attention](#attention)
	- [Install](#install)
	- [Event](#event)
	- [Quick Start](#quick-start)
	- [Best Practice](#best-practice)
	- [Usage](#usage)
		- [Upgrade from HTTP](#upgrade-from-http)
		- [Unix Domain Socket](#unix-domain-socket)
		- [Broadcast](#broadcast)
	- [Autobahn Test](#autobahn-test)
	- [Benchmark](#benchmark)
		- [Compression Disabled](#compression-disabled)
		- [Compression Enabled](#compression-enabled)
	- [Communication](#communication)
	- [Acknowledgments](#acknowledgments)

### Highlight

- IO multiplexing support, concurrent message processing and asynchronous non-blocking message writing
- High IOPS and low latency, low CPU usage
- Support fast parsing WebSocket protocol directly from TCP, faster handshake, lower memory usage
- Fully passes the WebSocket [autobahn-testsuite](https://lxzan.github.io/gws/reports/servers/)

### Attention

- The errors returned by the gws.Conn export methods are ignored, and are handled internally
- Transferring large files with gws tends to block the connection

### Install

```bash
go get -v github.com/lxzan/gws@latest
```

### Event

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

### Quick Start

```go
package main

import "github.com/lxzan/gws"

func main() {
	gws.NewServer(new(gws.BuiltinEventHandler), nil).Run(":6666")
}
```

### Best Practice

```go
package main

import (
	"github.com/lxzan/gws"
	"time"
)

const PingInterval = 10 * time.Second

func main() {
	options := &gws.ServerOption{ReadAsyncEnabled: true, ReadAsyncGoLimit: 4, CompressEnabled: true}
	gws.NewServer(new(Handler), options).Run(":6666")
}

type Handler struct{}

func (c *Handler) OnOpen(socket *gws.Conn) { _ = socket.SetDeadline(time.Now().Add(2 * PingInterval)) }

func (c *Handler) DeleteSession(socket *gws.Conn) {}

func (c *Handler) OnError(socket *gws.Conn, err error) { c.DeleteSession(socket) }

func (c *Handler) OnClose(socket *gws.Conn, code uint16, reason []byte) { c.DeleteSession(socket) }

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(2 * PingInterval))
	_ = socket.WritePong(nil)
}

func (c *Handler) OnPong(socket *gws.Conn, payload []byte) {}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
}
```

### Usage

#### Upgrade from HTTP

```go
package main

import (
	"github.com/lxzan/gws"
	"log"
	"net/http"
)

func main() {
	upgrader := gws.NewUpgrader(new(gws.BuiltinEventHandler), &gws.ServerOption{
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			session.Store("username", r.URL.Query().Get("username"))
			return true
		},
	})

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			log.Printf(err.Error())
			return
		}
		socket.ReadLoop()
	})

	if err := http.ListenAndServe(":6666", nil); err != nil {
		log.Fatalf("%v", err)
	}
}
```

#### Unix Domain Socket

- server

```go
package main

import (
	"log"
	"net"
	"github.com/lxzan/gws"
)

func main() {
	listener, err := net.Listen("unix", "/tmp/gws.sock")
	if err != nil {
		log.Println(err.Error())
		return
	}
	var app = gws.NewServer(new(gws.BuiltinEventHandler), nil)
	if err := app.RunListener(listener); err != nil {
		log.Println(err.Error())
	}
}
```

- client

```go
package main

import (
	"log"
	"net"
	"github.com/lxzan/gws"
)

func main() {
	conn, err := net.Dial("unix", "/tmp/gws.sock")
	if err != nil {
		log.Println(err.Error())
		return
	}

	option := gws.ClientOption{}
	socket, _, err := gws.NewClientFromConn(new(gws.BuiltinEventHandler), &option, conn)
	if err != nil {
		log.Println(err.Error())
		return
	}
	socket.ReadLoop()
}
```

#### Broadcast

```go
func Broadcast(conns []*gws.Conn, opcode gws.Opcode, payload []byte) {
	for _, item := range conns {
		_ = item.WriteAsync(opcode, payload)
	}
}
```

### Autobahn Test

```bash
cd examples/autobahn
mkdir reports
docker run -it --rm \
    -v ${PWD}/config:/config \
    -v ${PWD}/reports:/reports \
    crossbario/autobahn-testsuite \
    wstest -m fuzzingclient -s /config/fuzzingclient.json
```

### Benchmark

- Machine: `Ubuntu 20.04LTS VM (4C8T)`
- ConnectionNum: `1000`

#### Compression Disabled

| Package                       | Payload         | Duration | IOPS      | P50   | P90    | P99    |
| ----------------------------- | --------------- | -------- | --------- | ----- | ------ | ------ |
| lxzan/gws                     | rand.Intn(1000) | 712.62ms | 1,403,261 | 13ms  | 193ms  | 282ms  |
| lesismal/nbio (IOModBlocking) | rand.Intn(1000) | 803.06ms | 1,245,230 | 48ms  | 304ms  | 473ms  |
| gorilla/websocket             | rand.Intn(1000) | 1.43s    | 695,787   | 270ms | 807ms  | 1105ms |
| nhooyr/websocket              | rand.Intn(1000) | 2.80s    | 356,775   | 697ms | 1966ms | 2522ms |

#### Compression Enabled

| Package                       | Payload         | Duration | IOPS    | P50    | P90    | P99    |
| ----------------------------- | --------------- | -------- | ------- | ------ | ------ | ------ |
| lxzan/gws                     | rand.Intn(4000) | 1.73s    | 287,628 | 58ms   | 326ms  | 828ms  |
| lesismal/nbio (IOModBlocking) | rand.Intn(4000) | 2.08s    | 239,307 | 58ms   | 460ms  | 1206ms |
| gorilla/websocket             | rand.Intn(4000) | 4.91s    | 101,721 | 488ms  | 2276ms | 4235ms |
| nhooyr/websocket              | rand.Intn(4000) | 10.99s   | 45,467  | 1159ms | 5228ms | ∞      |

### Communication

<img src="assets/qq.jpg" alt="QQ" width="300"/>

### Acknowledgments

The following project had particular influence on gws's design.

- [lesismal/nbio](https://github.com/lesismal/nbio)
- [crossbario/autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite)

