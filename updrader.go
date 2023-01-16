package gws

import (
	"compress/flate"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	bpool = internal.NewBufferPool()
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

	// Upgrader websocket upgrader
	// do not use &Upgrader unless, some options may not be initialized, NewUpgrader is recommended
	Upgrader struct {
		once sync.Once

		// websocket event handler
		EventHandler Event

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

func NewUpgrader(options ...Option) *Upgrader {
	var c = new(Upgrader)
	options = append(options, withInitialize())
	for _, f := range options {
		f(c)
	}
	return c
}

func (c *Upgrader) connectHandshake(conn net.Conn, headers http.Header, websocketKey string) error {
	// handshake
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
	if _, err := conn.Write(buf); err != nil {
		return err
	}

	// initialize the connection
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}
	if err := setNoDelay(conn); err != nil {
		return err
	}
	return nil
}

// Accept http protocol upgrade to websocket
// ctx done means server stopping
func (c *Upgrader) Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	withInitialize()(c)
	var request = &Request{Request: r, SessionStorage: NewMap()}
	var header = internal.CloneHeader(c.ResponseHeader)
	if !c.CheckOrigin(request) {
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
	if val := r.Header.Get(internal.SecWebSocketExtensions); strings.Contains(val, "permessage-deflate") && c.CompressEnabled {
		header.Set(internal.SecWebSocketExtensions, "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
		compressEnabled = true
	}
	var websocketKey = r.Header.Get(internal.SecWebSocketKey)
	if websocketKey == "" {
		return nil, internal.ErrHandshake
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, internal.CloseInternalServerErr
	}
	netConn, brw, err := hj.Hijack()
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}
	if err := c.connectHandshake(netConn, header, websocketKey); err != nil {
		_ = netConn.Close()
		return nil, err
	}

	return serveWebSocket(c, request, netConn, brw, c.EventHandler, compressEnabled), nil
}
