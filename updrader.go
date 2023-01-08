package gws

import (
	"compress/flate"
	_ "embed"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	_pool = internal.NewBufferPool()
)

const (
	defaultCompressLevel    = flate.BestSpeed
	defaultMaxContentLength = 16 * 1024 * 1024 // 16MiB
)

type (
	Request struct {
		*http.Request                 // http request
		SessionStorage SessionStorage // store user session
	}

	Config struct {
		// whether to compress data
		CompressEnabled bool

		// compress level eg: flate.BestSpeed
		CompressLevel int

		// max message size
		MaxContentLength int

		// whether to check utf8 encoding, disabled for better performance
		CheckTextEncoding bool

		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		// attention: client may not support custom response header, use nil instead
		ResponseHeader http.Header

		// client authentication
		CheckOrigin func(r *Request) bool
	}
)

func (c *Config) initialize() {
	if c.ResponseHeader == nil {
		c.ResponseHeader = http.Header{}
	}
	if c.CheckOrigin == nil {
		c.CheckOrigin = func(r *Request) bool {
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
func Accept(w http.ResponseWriter, r *http.Request, eventHandler Event, config *Config) (*Conn, error) {
	if config == nil {
		config = new(Config)
	}
	config.initialize()

	var request = &Request{Request: r, SessionStorage: NewMap()}
	var header = internal.CloneHeader(config.ResponseHeader)
	if !config.CheckOrigin(request) {
		return nil, internal.ErrCheckOrigin
	}

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
		header.Set(internal.SecWebSocketExtensions, "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
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

	var websocketKey = r.Header.Get(internal.SecWebSocketKey)
	if err := handshake(netConn, header, websocketKey); err != nil {
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
	return serveWebSocket(config, request, netConn, brw, eventHandler, compressEnabled), nil
}
