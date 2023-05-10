package gws

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
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
			Addr:        "unix:///",
			DialTimeout: time.Second,
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
	var as = assert.New(t)
	option := &ClientOption{
		CompressEnabled: true,
		RequestHeader:   http.Header{},
	}
	option.RequestHeader.Set(internal.SecWebSocketKey.Key, "1fTfP/qALD+eAWcU80P0bg==")
	option.initialize()
	srv, cli := net.Pipe()
	var d = &dialer{
		option:          option,
		conn:            cli,
		eventHandler:    new(BuiltinEventHandler),
		resp:            &http.Response{Header: http.Header{}},
		secWebsocketKey: "1fTfP/qALD+eAWcU80P0bg==",
	}

	go func() {
		var text = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ygR8UkmG67DM75dkgZzwplwlEEo=\r\n\r\n"
		for {
			var buf = make([]byte, 1024)
			srv.Read(buf)
			srv.Write([]byte(text))
		}
	}()
	if _, _, err := d.handshake(); err != nil {
		as.NoError(err)
		return
	}
}

func TestClientHandshakeFail(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		option := &ClientOption{
			CompressEnabled: true,
			RequestHeader:   http.Header{},
		}
		option.RequestHeader.Set(internal.SecWebSocketKey.Key, "1fTfP/qALD+eAWcU80P0bg==")
		option.initialize()
		srv, cli := net.Pipe()
		var d = &dialer{
			option:          option,
			conn:            cli,
			secWebsocketKey: "1fTfP/qALD+eAWcU80P0bg==",
			eventHandler:    new(BuiltinEventHandler),
			resp:            &http.Response{Header: http.Header{}},
		}

		go func() {
			var text = "HTTP/1.1 400 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ygR8UkmG67DM75dkgZzwplwlEEo=\r\n\r\n"
			for {
				var buf = make([]byte, 1024)
				srv.Read(buf)
				srv.Write([]byte(text))
			}
		}()
		_, _, err := d.handshake()
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		option := &ClientOption{
			CompressEnabled: true,
			RequestHeader:   http.Header{},
		}
		option.RequestHeader.Set(internal.SecWebSocketKey.Key, "1fTfP/qALD+eAWcU80P0bg==")
		option.initialize()
		srv, cli := net.Pipe()
		var d = &dialer{
			option:          option,
			conn:            cli,
			secWebsocketKey: "1fTfP/qALD+eAWcU80P0bg==",
			eventHandler:    new(BuiltinEventHandler),
			resp:            &http.Response{Header: http.Header{}},
		}

		go func() {
			var text = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upg: rade\r\nSec-WebSocket-Accept: ygR8UkmG67DM75dkgZzwplwlEEo=\r\n\r\n"
			for {
				var buf = make([]byte, 1024)
				srv.Read(buf)
				srv.Write([]byte(text))
			}
		}()
		_, _, err := d.handshake()
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		option := &ClientOption{
			CompressEnabled: true,
			RequestHeader:   http.Header{},
		}
		option.initialize()
		srv, cli := net.Pipe()
		var d = &dialer{
			option:          option,
			conn:            cli,
			secWebsocketKey: "1fTfP/qALD+eAWcU80P0bg==",
			eventHandler:    new(BuiltinEventHandler),
			resp:            &http.Response{Header: http.Header{}},
		}

		go func() {
			var text = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: _ygR8UkmG67DM75dkgZzwplwlEEo=\r\n\r\n"
			for {
				var buf = make([]byte, 1024)
				srv.Read(buf)
				srv.Write([]byte(text))
			}
		}()
		_, _, err := d.handshake()
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		option := &ClientOption{
			CompressEnabled: true,
			RequestHeader:   http.Header{},
		}
		option.RequestHeader.Set(internal.SecWebSocketKey.Key, "1fTfP/qALD+eAWcU80P0bg==")
		option.initialize()
		srv, cli := net.Pipe()
		var d = &dialer{
			option:          option,
			conn:            cli,
			secWebsocketKey: "1fTfP/qALD+eAWcU80P0bg==",
			eventHandler:    new(BuiltinEventHandler),
			resp:            &http.Response{Header: http.Header{}},
		}

		go func() {
			var text = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket1\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ygR8UkmG67DM75dkgZzwplwlEEo=\r\n\r\n"
			for {
				var buf = make([]byte, 1024)
				srv.Read(buf)
				srv.Write([]byte(text))
			}
		}()
		_, _, err := d.handshake()
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		option := &ClientOption{
			CompressEnabled: true,
			RequestHeader:   http.Header{},
		}
		option.RequestHeader.Set(internal.SecWebSocketKey.Key, "1fTfP/qALD+eAWcU80P0bg==")
		option.initialize()
		srv, cli := net.Pipe()
		var d = &dialer{
			option:          option,
			conn:            cli,
			secWebsocketKey: "1fTfP/qALD+eAWcU80P0bg==",
			eventHandler:    new(BuiltinEventHandler),
			resp:            &http.Response{Header: http.Header{}},
		}

		go func() {
			var text = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket1\r\nConnection: Upgrade1\r\nSec-WebSocket-Accept: ygR8UkmG67DM75dkgZzwplwlEEo=\r\n\r\n"
			for {
				var buf = make([]byte, 1024)
				srv.Read(buf)
				srv.Write([]byte(text))
			}
		}()
		_, _, err := d.handshake()
		as.Error(err)
	})
}
