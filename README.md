# gws

### event-driven websocket server

[![Build Status](https://github.com/lxzan/gws/workflows/Go%20Test/badge.svg?branch=master)](https://github.com/lxzan/gws/actions?query=branch%3Amaster)

#### Highlight

- zero dependency, no channel, event driven
- zero extra goroutine to manage business logic
- zero error to read/write operation, errors have been handled appropriately
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
		ResponseHeader:    http.Header{"Server": []string{"gws"}},
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

#### Benchmark
```
// machine: 4C8T Ubuntu VM
// cmd: tcpkali -c 200 -r 20000 -f body.json -T 30s --ws 127.0.0.1:3000/connect
// body.json size: 2.4KiB
// max cost: cpu=420% memory=24MiB

Destination: [127.0.0.1]:3000
Interface lo address [127.0.0.1]:0
Using interface lo to connect to [127.0.0.1]:3000
Ramped up to 200 connections.
Total data sent:     36573.8 MiB (38350422218 bytes)
Total data received: 36529.6 MiB (38304064401 bytes)
Bandwidth per channel: 102.178⇅ Mbps (12772.2 kBps)
Aggregate bandwidth: 10211.581↓, 10223.939↑ Mbps
Packet rate estimate: 943099.1↓, 884811.7↑ (8↓, 43↑ TCP MSS/op)
Test duration: 30.0083 s.
```
