package gws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _port = int64(19999)

func nextPort() string {
	port := atomic.AddInt64(&_port, 1)
	return strconv.Itoa(int(port))
}

func newHttpWriter() *httpWriter {
	server, client := net.Pipe()
	var r = bytes.NewBuffer(nil)
	var w = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	go func() {
		for {
			var p [1024]byte
			if _, err := client.Read(p[0:]); err != nil {
				return
			}
		}
	}()

	return &httpWriter{
		conn: server,
		brw:  brw,
	}
}

type httpWriter struct {
	conn net.Conn
	brw  *bufio.ReadWriter
}

func (c *httpWriter) Header() http.Header {
	return http.Header{}
}

func (c *httpWriter) Write(i []byte) (int, error) {
	return 0, nil
}

func (c *httpWriter) WriteHeader(statusCode int) {}

func (c *httpWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return c.conn, c.brw, nil
}

type httpWriterWrapper1 struct {
	*httpWriter
}

func (c *httpWriterWrapper1) Hijack() {}

type httpWriterWrapper2 struct {
	*httpWriter
}

func (c *httpWriterWrapper2) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return c.conn, nil, errors.New("test")
}

func TestNoDelay(t *testing.T) {
	t.Run("tcp conn", func(t *testing.T) {
		conn := &Conn{conn: &net.TCPConn{}}
		conn.SetNoDelay(false)
	})

	t.Run("tls conn", func(t *testing.T) {
		tlsConn := tls.Client(&net.TCPConn{}, nil)
		conn := &Conn{conn: tlsConn}
		conn.SetNoDelay(false)
	})

	t.Run("other", func(t *testing.T) {
		conn, _ := net.Pipe()
		socket := &Conn{conn: conn}
		socket.SetNoDelay(false)
	})
}

func TestAccept(t *testing.T) {
	var upgrader = NewUpgrader(new(webSocketMocker), &ServerOption{
		PermessageDeflate: PermessageDeflate{Enabled: true},
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		ResponseHeader: http.Header{
			"Server": []string{"gws"},
		},
	})

	t.Run("ok", func(t *testing.T) {
		upgrader.option.PermessageDeflate.Enabled = true
		upgrader.option.SubProtocols = []string{"chat"}
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
		request.Header.Set("Sec-WebSocket-Protocol", "chat")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.NoError(t, err)
	})

	t.Run("fail Sec-WebSocket-Version", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "14")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "client_max_window_bits")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail method", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodPost,
		}
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail Connection", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "up")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail Connection", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "ws")
		request.Header.Set("Sec-WebSocket-Version", "13")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail Sec-WebSocket-Key", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail check origin", func(t *testing.T) {
		upgrader.option.PermessageDeflate.Enabled = true
		upgrader.option.Authorize = func(r *http.Request, session SessionStorage) bool {
			return false
		}
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})
}

func TestFailHijack(t *testing.T) {
	var upgrader = NewUpgrader(new(webSocketMocker), &ServerOption{
		ResponseHeader: http.Header{"Server": []string{"gws"}},
	})
	var request = &http.Request{
		Header: http.Header{},
		Method: http.MethodGet,
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
	request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
	_, err := upgrader.Upgrade(&httpWriterWrapper1{httpWriter: newHttpWriter()}, request)
	assert.Error(t, err)

	_, err = upgrader.Upgrade(&httpWriterWrapper2{httpWriter: newHttpWriter()}, request)
	assert.Error(t, err)
}

func TestNewServer(t *testing.T) {
	var as = assert.New(t)

	t.Run("ok", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		_, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
		})
		as.NoError(err)
	})

	t.Run("tls", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), nil)
		var dir = os.Getenv("PWD")
		go server.RunTLS(addr, dir+"/examples/wss/cert/server.crt", dir+"/examples/wss/cert/server.pem")
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		<-ctx.Done()
	})

	t.Run("fail 1", func(t *testing.T) {
		var addr = ":" + nextPort()
		var wg = sync.WaitGroup{}
		wg.Add(1)
		var server = NewServer(new(BuiltinEventHandler), nil)
		server.OnError = func(conn net.Conn, err error) {
			wg.Done()
		}
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, err := net.Dial("tcp", "localhost"+addr)
		as.NoError(err)
		var payload = fmt.Sprintf("POST ws://localhost%s HTTP/1.1\r\n\r\n", addr)
		client.Write([]byte(payload))
		wg.Wait()
	})

	t.Run("fail 2", func(t *testing.T) {
		var addr = ":" + nextPort()
		var wg = sync.WaitGroup{}
		wg.Add(1)
		var server = NewServer(new(BuiltinEventHandler), nil)
		server.OnError = func(conn net.Conn, err error) {
			wg.Done()
		}
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, err := net.Dial("tcp", "localhost"+addr)
		as.NoError(err)
		var payload = fmt.Sprintf("GET ws://localhost%s HTTP/1.1 GWS\r\n\r\n", addr)
		client.Write([]byte(payload))
		wg.Wait()
	})

	t.Run("fail 3", func(t *testing.T) {
		var server = NewServer(new(BuiltinEventHandler), nil)
		var addr = ":" + nextPort()
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		as.Error(NewServer(new(BuiltinEventHandler), nil).Run(addr))
		as.Error(NewServer(new(BuiltinEventHandler), nil).RunTLS(addr, "", ""))
		{
			server := NewServer(new(BuiltinEventHandler), nil)
			var dir = "./"
			go server.RunTLS(":"+nextPort(), dir+"/examples/wss/cert/server.crt", dir+"/examples/wss/cert/server.pem")
			time.Sleep(100 * time.Millisecond)
			as.Error(server.RunTLS(addr, dir+"/examples/wss/cert/server.crt", dir+"/examples/wss/cert/server.pem"))
		}
	})
}

func TestBuiltinEventEngine(t *testing.T) {
	{
		var ev = &webSocketMocker{}
		_, ok := any(ev).(Event)
		assert.Equal(t, true, ok)

		ev.OnOpen(nil)
		ev.OnClose(nil, nil)
		ev.OnMessage(nil, &Message{})
		ev.OnPing(nil, nil)
		ev.OnPong(nil, nil)
	}

	{
		var ev = &BuiltinEventHandler{}
		ev.OnMessage(nil, &Message{})
		ev.OnPong(nil, nil)
	}
}

func TestSubprotocol(t *testing.T) {
	t.Run("server close", func(t *testing.T) {
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{SubProtocols: []string{"chat"}})
		go func() { app.Run(addr) }()

		time.Sleep(100 * time.Millisecond)
		_, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{Addr: "ws://" + addr})
		assert.Error(t, err)
	})

	t.Run("client close", func(t *testing.T) {
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{})
		go func() { app.Run(addr) }()

		time.Sleep(100 * time.Millisecond)
		rh := http.Header{}
		rh.Set("Sec-WebSocket-Protocol", "chat")
		_, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr:          "ws://" + addr,
			RequestHeader: rh,
		})
		assert.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{SubProtocols: []string{"chat"}})
		go func() { app.Run(addr) }()

		time.Sleep(100 * time.Millisecond)
		rh := http.Header{}
		rh.Set("Sec-WebSocket-Protocol", "chat")
		_, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr:          "ws://" + addr,
			RequestHeader: rh,
		})
		assert.NoError(t, err)
	})
}

func TestResponseWriter_Write(t *testing.T) {
	t.Run("", func(t *testing.T) {
		conn, _ := net.Pipe()
		rw := &responseWriter{b: bytes.NewBuffer(nil)}
		conn.Close()
		err := rw.Write(conn, time.Second)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		conn, _ := net.Pipe()
		rw := &responseWriter{b: bytes.NewBuffer(nil)}
		err := rw.Write(conn, time.Nanosecond)
		assert.Error(t, err)
	})
}

func TestServer_RunListener(t *testing.T) {
	var s = NewServer(&BuiltinEventHandler{}, nil)
	var ln, _ = net.Listen("tcp", ":"+nextPort())
	_ = ln.Close()
	go func() {
		log.Default().SetOutput(bytes.NewBuffer(nil))
		s.RunListener(ln)
	}()
	time.Sleep(time.Microsecond)
}
