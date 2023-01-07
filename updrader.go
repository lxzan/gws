package gws

import (
	"compress/flate"
	"context"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultCompressLevel    = flate.BestSpeed
	defaultMaxContentLength = 16 * 1024 * 1024 // 16MiB
)

type (
	Config struct {
		// whether to compress data
		CompressEnabled bool

		// compress level eg: flate.BestSpeed
		CompressLevel int

		// max message size
		MaxContentLength int

		// whether to check utf8 encoding, disabled for better performance
		CheckTextEncoding bool

		// client authentication
		Authenticate func(r *internal.Request) bool
	}
)

func (c *Config) initialize() {
	if c.Authenticate == nil {
		c.Authenticate = func(r *internal.Request) bool {
			return true
		}
	}
	if c.MaxContentLength <= 0 {
		c.MaxContentLength = defaultMaxContentLength
	}
	if c.CompressEnabled && c.CompressLevel == 0 {
		c.CompressLevel = defaultCompressLevel
	}
}

func handshake(conn net.Conn, headers http.Header, websocketKey string) error {
	var buf = make([]byte, 0, 256)
	buf = append(buf, "HTTP/1.1 101 Switching Protocols\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Sec-WebSocket-Accept: "...)
	buf = append(buf, internal.ComputeAcceptKey(websocketKey)...)
	buf = append(buf, "\r\n"...)
	for k, _ := range headers {
		buf = append(buf, k...)
		buf = append(buf, ": "...)
		buf = append(buf, headers.Get(k)...)
		buf = append(buf, "\r\n"...)
	}
	buf = append(buf, "\r\n"...)
	_, err := conn.Write(buf)
	return err
}

// Accept http protocol upgrade to websocket
// ctx done means server stopping
func Accept(ctx context.Context, w http.ResponseWriter, r *http.Request, eventHandler Event, config Config) (*Conn, error) {
	config.initialize()

	var request = &internal.Request{Request: r, SessionStorage: internal.NewMap()}
	var headers = http.Header{}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, errors.New("http method must be get")
	}
	if version := r.Header.Get(internal.SecWebSocketVersion); version != internal.SecWebSocketVersion_Value {
		msg := "websocket protocol not supported: " + version
		return nil, errors.New(msg)
	}
	if val := r.Header.Get(internal.Connection); strings.ToLower(val) != strings.ToLower(internal.Connection_Value) {
		return nil, internal.ErrHandshake
	}
	if val := r.Header.Get(internal.Upgrade); strings.ToLower(val) != internal.Upgrade_Value {
		return nil, internal.ErrHandshake
	}
	if val := r.Header.Get(internal.SecWebSocketExtensions); strings.Contains(val, "permessage-deflate") && config.CompressEnabled {
		headers.Set(internal.SecWebSocketExtensions, "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
		compressEnabled = true
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, internal.CloseInternalServerErr
	}
	netConn, brw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	if !config.Authenticate(request) {
		return nil, internal.ErrAuthenticate
	}

	var websocketKey = r.Header.Get(internal.SecWebSocketKey)
	if err := handshake(netConn, headers, websocketKey); err != nil {
		return nil, err
	}
	if err := netConn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if err := netConn.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if err := netConn.(*net.TCPConn).SetNoDelay(false); err != nil {
		return nil, err
	}
	return serveWebSocket(ctx, config, request, netConn, brw, eventHandler, compressEnabled), nil
}
