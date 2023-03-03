package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"testing"
)

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
	t.Run("tls conn", func(t *testing.T) {
		setNoDelay(&tls.Conn{})
	})

	t.Run("other", func(t *testing.T) {
		conn, _ := net.Pipe()
		setNoDelay(conn)
	})
}

func TestAccept(t *testing.T) {
	var upgrader = NewUpgrader(new(webSocketMocker), &ServerOption{
		ResponseHeader: http.Header{
			"Server": []string{"gws"},
		},
	})

	t.Run("ok", func(t *testing.T) {
		upgrader.option.CompressEnabled = true
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
		_, err := upgrader.Accept(newHttpWriter(), request)
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
		_, err := upgrader.Accept(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail method", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodPost,
		}
		_, err := upgrader.Accept(newHttpWriter(), request)
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
		_, err := upgrader.Accept(newHttpWriter(), request)
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
		_, err := upgrader.Accept(newHttpWriter(), request)
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
		_, err := upgrader.Accept(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail check origin", func(t *testing.T) {
		upgrader.option.CompressEnabled = true
		upgrader.option.CheckOrigin = func(r *Request) bool {
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
		_, err := upgrader.Accept(newHttpWriter(), request)
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
	_, err := upgrader.Accept(&httpWriterWrapper1{httpWriter: newHttpWriter()}, request)
	assert.Error(t, err)

	_, err = upgrader.Accept(&httpWriterWrapper2{httpWriter: newHttpWriter()}, request)
	assert.Error(t, err)
}

func TestBuiltinEventEngine(t *testing.T) {
	var ev = new(BuiltinEventHandler)
	_, ok := interface{}(ev).(Event)
	assert.Equal(t, true, ok)

	ev.OnError(nil, nil)
	ev.OnClose(nil, 0, nil)
	ev.OnMessage(nil, nil)
	ev.OnPing(nil, nil)
	ev.OnPong(nil, nil)
}
