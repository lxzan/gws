package gws

import (
	"crypto/tls"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	NewClient(new(BuiltinEventHandler), nil)
	{
		var option = &ClientOption{
			Addr: "ws://127.0.0.1",
		}
		NewClient(new(BuiltinEventHandler), option)
	}

	{
		var option = &ClientOption{
			Addr: "wss://127.0.0.1",
		}
		NewClient(new(BuiltinEventHandler), option)
	}

	{
		var option = &ClientOption{
			Addr: "unix:///",
		}
		NewClient(new(BuiltinEventHandler), option)
	}

	{
		var option = &ClientOption{
			Addr: "tls://127.0.0.1",
		}
		NewClient(new(BuiltinEventHandler), option)
	}
}

func TestNewClientFromConn(t *testing.T) {
	var as = assert.New(t)

	t.Run("ok", func(t *testing.T) {
		server := NewServer(BuiltinEventHandler{}, nil)
		addr := ":" + nextPort()
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		conn, err := net.Dial("tcp", "localhost"+addr)
		if err != nil {
			as.NoError(err)
			return
		}
		_, _, err = NewClientFromConn(BuiltinEventHandler{}, nil, conn)
		as.NoError(err)
	})

	t.Run("fail", func(t *testing.T) {
		server := NewServer(BuiltinEventHandler{}, nil)
		addr := ":" + nextPort()
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		conn, _ := net.Pipe()
		_, _, err := NewClientFromConn(BuiltinEventHandler{}, &ClientOption{}, conn)
		as.Error(err)
	})
}

func TestClientHandshake(t *testing.T) {
	var as = assert.New(t)
	option := &ClientOption{
		CompressEnabled: true,
		RequestHeader:   http.Header{},
	}
	option.RequestHeader.Set(internal.SecWebSocketKey.Key, "1fTfP/qALD+eAWcU80P0bg==")
	option = initClientOption(option)
	srv, cli := net.Pipe()
	var d = &connector{
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
		option = initClientOption(option)
		srv, cli := net.Pipe()
		var d = &connector{
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
		option = initClientOption(option)
		srv, cli := net.Pipe()
		var d = &connector{
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
		option = initClientOption(option)
		srv, cli := net.Pipe()
		var d = &connector{
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
		option = initClientOption(option)
		srv, cli := net.Pipe()
		var d = &connector{
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
		option = initClientOption(option)
		srv, cli := net.Pipe()
		var d = &connector{
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

var rsaCertPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUJeohtgk8nnt8ofratXJg7kUJsI4wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMDEyMDcwODIyNThaFw0zMDEy
MDUwODIyNThaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCy+ZrIvwwiZv4bPmvKx/637ltZLwfgh3ouiEaTchGu
IQltthkqINHxFBqqJg44TUGHWthlrq6moQuKnWNjIsEc6wSD1df43NWBLgdxbPP0
x4tAH9pIJU7TQqbznjDBhzRbUjVXBIcn7bNknY2+5t784pPF9H1v7h8GqTWpNH9l
cz/v+snoqm9HC+qlsFLa4A3X9l5v05F1uoBfUALlP6bWyjHAfctpiJkoB9Yw1TJa
gpq7E50kfttwfKNkkAZIbib10HugkMoQJAs2EsGkje98druIl8IXmuvBIF6nZHuM
lt3UIZjS9RwPPLXhRHt1P0mR7BoBcOjiHgtSEs7Wk+j7AgMBAAGjUzBRMB0GA1Ud
DgQWBBQdheJv73XSOhgMQtkwdYPnfO02+TAfBgNVHSMEGDAWgBQdheJv73XSOhgM
QtkwdYPnfO02+TAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBf
SKVNMdmBpD9m53kCrguo9iKQqmhnI0WLkpdWszc/vBgtpOE5ENOfHGAufHZve871
2fzTXrgR0TF6UZWsQOqCm5Oh3URsCdXWewVMKgJ3DCii6QJ0MnhSFt6+xZE9C6Hi
WhcywgdR8t/JXKDam6miohW8Rum/IZo5HK9Jz/R9icKDGumcqoaPj/ONvY4EUwgB
irKKB7YgFogBmCtgi30beLVkXgk0GEcAf19lHHtX2Pv/lh3m34li1C9eBm1ca3kk
M2tcQtm1G89NROEjcG92cg+GX3GiWIjbI0jD1wnVy2LCOXMgOVbKfGfVKISFt0b1
DNn00G8C6ttLoGU2snyk
-----END CERTIFICATE-----
`)

var rsaKeyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAsvmayL8MImb+Gz5rysf+t+5bWS8H4Id6LohGk3IRriEJbbYZ
KiDR8RQaqiYOOE1Bh1rYZa6upqELip1jYyLBHOsEg9XX+NzVgS4HcWzz9MeLQB/a
SCVO00Km854wwYc0W1I1VwSHJ+2zZJ2Nvube/OKTxfR9b+4fBqk1qTR/ZXM/7/rJ
6KpvRwvqpbBS2uAN1/Zeb9ORdbqAX1AC5T+m1soxwH3LaYiZKAfWMNUyWoKauxOd
JH7bcHyjZJAGSG4m9dB7oJDKECQLNhLBpI3vfHa7iJfCF5rrwSBep2R7jJbd1CGY
0vUcDzy14UR7dT9JkewaAXDo4h4LUhLO1pPo+wIDAQABAoIBAF6yWwekrlL1k7Xu
jTI6J7hCUesaS1yt0iQUzuLtFBXCPS7jjuUPgIXCUWl9wUBhAC8SDjWe+6IGzAiH
xjKKDQuz/iuTVjbDAeTb6exF7b6yZieDswdBVjfJqHR2Wu3LEBTRpo9oQesKhkTS
aFF97rZ3XCD9f/FdWOU5Wr8wm8edFK0zGsZ2N6r57yf1N6ocKlGBLBZ0v1Sc5ShV
1PVAxeephQvwL5DrOgkArnuAzwRXwJQG78L0aldWY2q6xABQZQb5+ml7H/kyytef
i+uGo3jHKepVALHmdpCGr9Yv+yCElup+ekv6cPy8qcmMBqGMISL1i1FEONxLcKWp
GEJi6QECgYEA3ZPGMdUm3f2spdHn3C+/+xskQpz6efiPYpnqFys2TZD7j5OOnpcP
ftNokA5oEgETg9ExJQ8aOCykseDc/abHerYyGw6SQxmDbyBLmkZmp9O3iMv2N8Pb
Nrn9kQKSr6LXZ3gXzlrDvvRoYUlfWuLSxF4b4PYifkA5AfsdiKkj+5sCgYEAzseF
XDTRKHHJnzxZDDdHQcwA0G9agsNj64BGUEjsAGmDiDyqOZnIjDLRt0O2X3oiIE5S
TXySSEiIkxjfErVJMumLaIwqVvlS4pYKdQo1dkM7Jbt8wKRQdleRXOPPN7msoEUk
Ta9ZsftHVUknPqblz9Uthb5h+sRaxIaE1llqDiECgYATS4oHzuL6k9uT+Qpyzymt
qThoIJljQ7TgxjxvVhD9gjGV2CikQM1Vov1JBigj4Toc0XuxGXaUC7cv0kAMSpi2
Y+VLG+K6ux8J70sGHTlVRgeGfxRq2MBfLKUbGplBeDG/zeJs0tSW7VullSkblgL6
nKNa3LQ2QEt2k7KHswryHwKBgENDxk8bY1q7wTHKiNEffk+aFD25q4DUHMH0JWti
fVsY98+upFU+gG2S7oOmREJE0aser0lDl7Zp2fu34IEOdfRY4p+s0O0gB+Vrl5VB
L+j7r9bzaX6lNQN6MvA7ryHahZxRQaD/xLbQHgFRXbHUyvdTyo4yQ1821qwNclLk
HUrhAoGAUtjR3nPFR4TEHlpTSQQovS8QtGTnOi7s7EzzdPWmjHPATrdLhMA0ezPj
Mr+u5TRncZBIzAZtButlh1AHnpN/qO3P0c0Rbdep3XBc/82JWO8qdb5QvAkxga3X
BpA7MNLxiqss+rCbwf3NbWxEMiDQ2zRwVoafVFys7tjmv6t2Xck=
-----END RSA PRIVATE KEY-----
`)

func TestNewClientWSS(t *testing.T) {
	var as = assert.New(t)

	addr := "127.0.0.1:" + nextPort()
	srv := NewServer(new(BuiltinEventHandler), nil)
	certs, _ := tls.X509KeyPair(rsaCertPEM, rsaKeyPEM)
	listener, err := tls.Listen("tcp", addr, &tls.Config{Certificates: []tls.Certificate{certs}})
	if err != nil {
		as.NoError(err)
		return
	}
	go func() {
		if err := srv.RunListener(listener); err != nil {
			as.NoError(err)
			return
		}
	}()

	t.Run("", func(t *testing.T) {
		_, _, err = NewClient(&BuiltinEventHandler{}, &ClientOption{
			Addr:      "wss://" + addr,
			TlsConfig: &tls.Config{InsecureSkipVerify: true},
		})
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		_, _, err = NewClient(&BuiltinEventHandler{}, &ClientOption{
			Addr:      "wss://" + addr,
			TlsConfig: &tls.Config{InsecureSkipVerify: true},
			NewDialer: func() (Dialer, error) {
				return nil, io.EOF
			},
		})
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		_, _, err = NewClient(&BuiltinEventHandler{}, &ClientOption{
			Addr:      "wss://" + addr + "/%x",
			TlsConfig: &tls.Config{InsecureSkipVerify: true},
			NewDialer: func() (Dialer, error) {
				return nil, io.EOF
			},
		})
		as.Error(err)
	})
}

func TestNewClient_WriteRequest(t *testing.T) {
	c := connector{option: &ClientOption{Addr: "ws://127.0.0.1/a=%"}}
	_, err := c.writeRequest()
	assert.Error(t, err)
}
