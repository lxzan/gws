package gws

import (
	"net"
	"net/url"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	NewClient(new(BuiltinEventHandler), nil)
	{
		var option = &ClientOption{
			Addr:        "ws://127.0.0.1",
			DialTimeout: time.Millisecond,
		}
		NewClient(new(BuiltinEventHandler), option)
	}

	{
		var option = &ClientOption{
			Addr:        "wss://127.0.0.1",
			DialTimeout: time.Millisecond,
		}
		NewClient(new(BuiltinEventHandler), option)
	}

	{
		var option = &ClientOption{
			Addr:        "tls://127.0.0.1",
			DialTimeout: time.Millisecond,
		}
		NewClient(new(BuiltinEventHandler), option)
	}
}

func TestClientHandshake(t *testing.T) {
	option := new(ClientOption)
	option.initialize()
	cli, srv := net.Pipe()
	u, _ := url.Parse("ws://127.0.0.1:3000")
	var d = &dialer{
		option:       option,
		conn:         cli,
		host:         "127.0.0.1:3000",
		u:            u,
		eventHandler: new(BuiltinEventHandler),
	}

	go func() {
		var text = `HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: LghwTcXaQ3YTNi3nHWs6qr3EWck=\r\n\r\n`
		go srv.Write([]byte(text))
	}()
	go func() {
		c, h, e := d.handshake()
		println(&c, &h, &e)
	}()
}
