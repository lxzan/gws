# gws

### event-driven websocket server

[![Build Status](https://github.com/lxzan/gws/workflows/Go%20Test/badge.svg?branch=master)](https://github.com/lxzan/gws/actions?query=branch%3Amaster)

#### Highlight

- zero dependency
- zero extra goroutine to control websocket
- zero error to read/write message, errors have been handled appropriately
- event driven
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
// machine: 2C4T Ubuntu VM
// cmd: tcpkali -c 100 -r 20000 -f body.json -T 20s --ws 127.0.0.1:3000/connect
// body.json size: 2.4KiB
// max cost: cpu=240% memory=18MiB

Destination: [127.0.0.1]:3000
Interface lo address [127.0.0.1]:0
Using interface lo to connect to [127.0.0.1]:3000
Ramped up to 100 connections.
Total data sent:     18199.2 MiB (19083295616 bytes)
Total data received: 18230.1 MiB (19115689063 bytes)
Bandwidth per channel: 152.724⇅ Mbps (19090.5 kBps)
Aggregate bandwidth: 7642.664↓, 7629.713↑ Mbps
Packet rate estimate: 688912.4↓, 666989.2↑ (8↓, 42↑ TCP MSS/op)
Test duration: 20.0095 s.
```
