package gws

import (
	"crypto/tls"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"testing"
)

func TestNoDelay(t *testing.T) {
	var as = assert.New(t)

	t.Run("tcp conn", func(t *testing.T) {
		setNoDelay(&net.TCPConn{})
	})

	t.Run("tls conn", func(t *testing.T) {
		tlsConn := &tls.Conn{}
		setNoDelay(tlsConn)
	})

	t.Run("other", func(t *testing.T) {
		conn, _ := net.Pipe()
		as.NoError(setNoDelay(conn))
	})
}

func TestAccept(t *testing.T) {
	var request = &http.Request{
		Header: http.Header{},
		Method: http.MethodGet,
	}

	t.Run("ok", func(t *testing.T) {
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "client_max_window_bits")
		_, err := Accept(newHttpWriter(), request, new(webSocketMocker), &Config{
			ResponseHeader: http.Header{"Server": []string{"gws"}},
		})
		assert.NoError(t, err)
	})

	t.Run("fail", func(t *testing.T) {
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "14")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "client_max_window_bits")
		_, err := Accept(newHttpWriter(), request, new(webSocketMocker), nil)
		assert.Error(t, err)
	})
}
