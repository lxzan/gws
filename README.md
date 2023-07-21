# gws

### event-driven go websocket server & client

[![Mentioned in Awesome Go][11]][12] [![Build Status][1]][2] [![MIT licensed][3]][4] [![Go Version][5]][6] [![codecov][7]][8] [![Go Report Card][9]][10]

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

[11]: https://awesome.re/mentioned-badge-flat.svg

[12]: https://github.com/avelino/awesome-go#networking

- [gws](#gws)
	- [Feature](#feature)
	- [Attention](#attention)
	- [Install](#install)
	- [Event](#event)
	- [Quick Start](#quick-start)
	- [Best Practice](#best-practice)
	- [Usage](#usage)
		- [Upgrade from HTTP](#upgrade-from-http)
		- [Unix Domain Socket](#unix-domain-socket)
		- [Client Proxy](#client-proxy)
		- [Broadcast](#broadcast)
	- [Autobahn Test](#autobahn-test)
	- [Benchmark](#benchmark)
	- [Communication](#communication)
	- [Acknowledgments](#acknowledgments)

### Feature

- [x] fully passes the websocket [autobahn-testsuite](https://lxzan.github.io/gws/reports/servers/)
- [x] thread safety guarantees for writing messages
- [x] high iops and low latency, low cpu usage
- [x] io multiplexing support, concurrent message processing and asynchronous non-blocking message writing
- [x] fast upgrade from tcp to websocket, dramatically reduce memory usage
- [x] create client via proxy

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
	OnClose(socket *Conn, err error)
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

func (c *Handler) OnClose(socket *gws.Conn, err error) {}

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

#### Client Proxy

```go
package main

import (
	"crypto/tls"
	"github.com/lxzan/gws"
	"golang.org/x/net/proxy"
	"log"
)

func main() {
	socket, _, err := gws.NewClient(new(gws.BuiltinEventHandler), &gws.ClientOption{
		Addr:      "wss://example.com/connect",
		TlsConfig: &tls.Config{InsecureSkipVerify: true},
		NewDialer: func() (gws.Dialer, error) {
			return proxy.SOCKS5("tcp", "127.0.0.1:1080", nil, nil)
		},
	})
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
	var b = gws.NewBroadcaster(opcode, payload)
	defer b.Release()
	for _, item := range conns {
		_ = b.Broadcast(item)
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

- GOMAXPROCS = 2
- Connection = 1000
- Compress Disabled
  
![performance](assets/performance-compress-disabled.png)

```go
$ go test -benchmem -run=^$ -bench ^(BenchmarkConn_WriteMessage|BenchmarkConn_ReadMessage)$ github.com/lxzan/gws

goos: darwin
goarch: arm64
pkg: github.com/lxzan/gws
BenchmarkConn_WriteMessage/compress_disabled-8         	 4494459	       239.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkConn_WriteMessage/compress_enabled-8          	  107365	     10726 ns/op	     509 B/op	       0 allocs/op
BenchmarkConn_ReadMessage/compress_disabled-8          	 3037701	       395.6 ns/op	     120 B/op	       3 allocs/op
BenchmarkConn_ReadMessage/compress_enabled-8           	  175388	      6355 ns/op	    7803 B/op	       7 allocs/op
PASS
ok  	github.com/lxzan/gws	5.813s
```

### Communication
> 微信二维码在讨论区不定时更新 

<div>
<img src="assets/wechat.jpg" alt="WeChat" width="300" height="300" style="display: inline-block;"/>
<span>&nbsp;&nbsp;&nbsp;&nbsp;</span>
<img src="assets/qq.jpg" alt="QQ" width="300" height="300" style="display: inline-block"/>
</div>


### Acknowledgments

The following project had particular influence on gws's design.

- [lesismal/nbio](https://github.com/lesismal/nbio)
- [crossbario/autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite)
