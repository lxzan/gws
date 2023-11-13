<p align="center"><b><font size="24">GWS</font></b></p>

<p align="center"> <img src="assets/gws_logo.png" alt="GWS" width="400" height="400"></p>

<h3 align="center">Simple, Fast, Reliable WebSocket Server & Client</h3>

<p align="center">
    <a href="https://github.com/avelino/awesome-go#networking">
        <img src="https://awesome.re/mentioned-badge-flat.svg" alt="">
    </a>
    <a href="https://github.com/lxzan/gws/actions?query=branch%3Amaster">
        <img src="https://github.com/lxzan/gws/workflows/Go%20Test/badge.svg?branch=master" alt="">
    </a>
    <a href="https://app.codecov.io/gh/lxzan/gws">
        <img src="https://codecov.io/github/lxzan/gws/branch/master/graph/badge.svg?token=DJU7YXWN05" alt="">
    </a>
    <a href="https://goreportcard.com/report/github.com/lxzan/gws">
        <img src="https://goreportcard.com/badge/github.com/lxzan/gws" alt="">
    </a>
    <a href="LICENSE">
        <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="">
    </a>
    <a href="https://github.com/lxzan/gws">
        <img src="https://img.shields.io/badge/go-%3E%3D1.18-30dff3?style=flat-square&logo=go" alt="">
    </a>
</p>

### Introduction

GWS (Go WebSocket) is a very simple, fast, reliable and fully-featured WebSocket implementation written in Go. It is
designed to be used in highly-concurrent environments, and it is suitable for
building `API`, `PROXY`, `GAME`, `Live Video`, `MESSAGE`, etc. It supports both server and client side with a simple API
which mean you can easily write a server or client by yourself.

GWS developed base on Event-Driven model. every connection has a goroutine to handle the event, and the event is able
to be processed in a non-blocking way.

### Why GWS

- <font size=4>Simplicity and Ease of Use</font>

  - **User-Friendly API**: Straightforward and easy-to-understand API, making server and client setup hassle-free.
  - **Code Efficiency**: Minimizes the amount of code needed to implement complex WebSocket solutions.

- <font size=4>High-Performance</font>

  - **Zero Allocs IO**: Built-in multi-level memory pool to minimize dynamic memory allocation during reads and
    writes.
  - **Optimized for Speed**: Designed for rapid data transmission and reception, ideal for time-sensitive
    applications.

- <font size=4>Reliability and Stability</font>

  - **Event-Driven Architecture**: Ensures stable performance even in highly concurrent environments.
  - **Robust Error Handling**: Advanced mechanisms to manage and mitigate errors, ensuring continuous operation.

- <font size=4>Versatility in Application</font>
  - **Wide Range of Use Cases**: Suitable for APIs, proxy servers, gaming, live video streaming, messaging, and more.
  - **Cross-Platform Compatibility**: Seamless integration across various platforms and environments.

### Attention

- Errors produced by `gws.Conn` export methods are internally resolved without external exposure.
- Large file transfers in GWS may lead to connection blockages.
- If HTTP Server reused in GWS, activating goroutines is suggested to avoid blocking issues that hinder effective garbage collection.

### Protocol Compliance

The GWS package passes the server tests in the [Autobahn TestSuite](https://github.com/crossbario/autobahn-testsuite) using the application in the [autobahn](https://github.com/lxzan/gws/tree/master/autobahn)

**Autobahn Test**

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

#### IOPS (Echo Server)

GOMAXPROCS=4, Connection=1000, CompressEnabled=false

> Gorilla and Nhooyr not using Streaming API

![performance](assets/performance-compress-disabled.png)

Ohh!!!, IOPS is `270K`+ with `8K` message size. It's amazing.

#### GoBench

```go
goos: linux
goarch: amd64
pkg: github.com/lxzan/gws
cpu: AMD Ryzen 5 PRO 4650G with Radeon Graphics
BenchmarkConn_WriteMessage/compress_disabled-8         	 7252513	       165.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkConn_WriteMessage/compress_enabled-8          	   97394	     10391 ns/op	     349 B/op	       0 allocs/op
BenchmarkConn_ReadMessage/compress_disabled-8          	 7812108	       152.3 ns/op	      16 B/op	       0 allocs/op
BenchmarkConn_ReadMessage/compress_enabled-8           	  368712	      3248 ns/op	     108 B/op	       0 allocs/op
PASS
```

## Install

```bash
go get -v github.com/lxzan/gws@latest
```

### Quick Start

Very, very, very simple example. **(Don't use it in production)**

The example let you know how to use the `gws` package without any other dependencies.

```go
package main

import "github.com/lxzan/gws"

func main() {
	gws.NewServer(new(gws.BuiltinEventHandler), nil).Run(":6666")
}
```

### Best Practice

#### Event

Event struct is used to handle the event of the connection.

- [x] **OnOpen** is called when the connection is established.
- [x] **OnClose** is called when the connection is closed.
- [x] **OnPing** is called when the connection send a ping control message.
- [x] **OnPong** is called when the connection receive a pong control message.
- [x] **OnMessage** is called when the connection receive a message.

```go
type Event interface {
    OnOpen(socket *Conn)                        // the connection is established
    OnClose(socket *Conn, err error)            // received a close frame or I/O error occurs
    OnPing(socket *Conn, payload []byte)        // receive a ping frame
    OnPong(socket *Conn, payload []byte)        // receive a pong frame
    OnMessage(socket *Conn, message *Message)   // receive a text/binary frame
}
```

#### Server

Frist of all, you need to implement the `Event` interface.

The Event implementation will be used in `gws.NewUpgrader`, a `*gws.Upgrader` instance be created and bind the `http.Handler` to the `http.Server`.

Http server will be started by `http.ListenAndServe`.

```go
package main

import (
	"github.com/lxzan/gws"
	"net/http"
	"time"
)

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

func main() {
	upgrader := gws.NewUpgrader(&Handler{}, &gws.ServerOption{
		ReadAsyncEnabled: true,
		CompressEnabled:  true,
		Recovery:         gws.Recovery,
	})
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			return
		}
		go func() {
			// Blocking prevents the context from being GC.
			socket.ReadLoop()
		}()
	})
	http.ListenAndServe(":6666", nil)
}

type Handler struct{}

func (c *Handler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WritePong(nil)
}

func (c *Handler) OnPong(socket *gws.Conn, payload []byte) {}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	socket.WriteMessage(message.Opcode, message.Bytes())
}
```

#### Client

Like the server, you need to implement the `Event` interface.

The Event implementation in `gws.NewClient`, a `*gws.Conn` instance be created and run it.

```go
package main

import (
	"github.com/lxzan/gws"
	"net/http"
	"time"
)

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

func main() {
	socket, _, err := gws.NewClient(&Handler{}, &gws.ClientOption{
		Addr: "ws://127.0.0.1:3000/connect",
	})
	if err != nil {
		log.Printf(err.Error())
		return
	}
	go socket.ReadLoop()

	for {
		var text = ""
		fmt.Scanf("%s", &text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		socket.WriteString(text)
	}
}

type Handler struct{}

func (c *Handler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WritePong(nil)
}

func (c *Handler) OnPong(socket *gws.Conn, payload []byte) {}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	socket.WriteMessage(message.Opcode, message.Bytes())
}
```

### More Examples

#### KCP

KCP: A Fast and Reliable ARQ Protocol, [kcp-go](github.com/xtaci/kcp-go) is a Production-Grade Reliable-UDP Library for golang.

- **server**

the `kcp-go` package is used to create a listener. The `gws` package is used to create a server with listener.

```go
package main

import (
	"log"
	"github.com/lxzan/gws"
	kcp "github.com/xtaci/kcp-go"
)

func main() {
	listener, err := kcp.Listen(":6666")
	if err != nil {
		log.Println(err.Error())
		return
	}
	app := gws.NewServer(&gws.BuiltinEventHandler{}, nil)
	app.RunListener(listener)
}
```

- **client**

the `kcp-go` package is used to create a connection. The `gws` package is used to create a client with connection.

```go
package main

import (
	"github.com/lxzan/gws"
	kcp "github.com/xtaci/kcp-go"
	"log"
)

func main() {
	conn, err := kcp.Dial("127.0.0.1:6666")
	if err != nil {
		log.Println(err.Error())
		return
	}
	app, _, err := gws.NewClientFromConn(&gws.BuiltinEventHandler{}, nil, conn)
	if err != nil {
		log.Println(err.Error())
		return
	}
	app.ReadLoop()
}
```

#### Proxy

The gws client sets the proxy server. Here is an example of using the `SOCKS5` proxy server.

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

The `gws` package provides a `*gws.Broadcaster` to broadcast messages to multiple connections.

```go
func Broadcast(conns []*gws.Conn, opcode gws.Opcode, payload []byte) {
	var b = gws.NewBroadcaster(opcode, payload)
	defer b.Close()
	for _, item := range conns {
		_ = b.Broadcast(item)
	}
}
```

### Contact

If you have questions, feel free to reach out to us in the following ways:

<div>
<img src="assets/wechat.png" alt="WeChat" width="300" height="300" style="display: inline-block;"/>
<span>&nbsp;&nbsp;&nbsp;&nbsp;</span>
<img src="assets/qq.jpg" alt="QQ" width="300" height="300" style="display: inline-block"/>
</div>

> 微信需要先添加好友, 然后拉人入群, 请注明来意.

### Thanks to

The following project had particular influence on gws's design.

- [crossbario/autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite)
- [klauspost/compress](https://github.com/klauspost/compress)
- [lesismal/nbio](https://github.com/lesismal/nbio)
