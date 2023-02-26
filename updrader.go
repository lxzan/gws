package gws

import (
	"compress/flate"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAsyncIOGoLimit       = 8
	defaultCompressLevel        = flate.BestSpeed
	defaultMaxContentLength     = 16 * 1024 * 1024 // 16MiB
	defaultCompressionThreshold = 512              // 512 Byte
	defaultCloseTimeout         = time.Second      // 1s
)

type (
	Request struct {
		*http.Request                 // http request
		SessionStorage SessionStorage // store user session
	}

	// Upgrader websocket upgrader
	Upgrader struct {
		// websocket event handler
		EventHandler Event

		// whether to enable asynchronous reading. if on, onmessage will be called concurrently.
		AsyncReadEnabled bool

		// goroutine limits on concurrent async io
		AsyncIOGoLimit int

		// maximum wait time to close a connection. for fast closing of abnormal connections, do not set this value too high.
		// when an IO exception occurs, there may be some unwritten data in the connection, so you need to wait
		CloseTimeout time.Duration

		// whether to compress data
		CompressEnabled bool

		// compress level eg: flate.BestSpeed
		CompressLevel int

		// if contentLength < compressionThreshold, it won't be compressed.
		CompressionThreshold int

		// max message size
		MaxContentLength int

		// whether to check utf8 encoding when read messages, disabled for better performance
		CheckTextEncoding bool

		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		// attention: client may not support custom response header, use nil instead
		ResponseHeader http.Header

		// client authentication
		CheckOrigin func(r *Request) bool
	}
)

// Initialize the upgrader configure
// 如果没有使用NewUpgrader, 需要调用此方法初始化配置
func (c *Upgrader) Initialize() {
	if c.EventHandler == nil {
		c.EventHandler = new(BuiltinEventHandler)
	}
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
	if c.CompressionThreshold <= 0 {
		c.CompressionThreshold = defaultCompressionThreshold
	}
	if c.AsyncIOGoLimit <= 0 {
		c.AsyncIOGoLimit = defaultAsyncIOGoLimit
	}
	if c.CloseTimeout <= 0 {
		c.CloseTimeout = defaultCloseTimeout
	}
}

func NewUpgrader(options ...Option) *Upgrader {
	var c = new(Upgrader)
	for _, f := range options {
		f(c)
	}
	c.Initialize()
	return c
}

func (c *Upgrader) connectHandshake(conn net.Conn, headers http.Header, websocketKey string) error {
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

// Accept http upgrade to websocket protocol
func (c *Upgrader) Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	socket, err := c.doAccept(w, r)
	if err != nil {
		if socket != nil && socket.conn != nil {
			_ = socket.conn.Close()
		}
		return nil, err
	}
	return socket, err
}

func (c *Upgrader) doAccept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	var request = &Request{Request: r, SessionStorage: NewMap()}
	var header = internal.CloneHeader(c.ResponseHeader)
	if !c.CheckOrigin(request) {
		return nil, internal.ErrCheckOrigin
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, internal.ErrGetMethodRequired
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

	// Hijack
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, internal.CloseInternalServerErr
	}
	netConn, brw, err := hj.Hijack()
	if err != nil {
		return &Conn{conn: netConn}, err
	}
	if err := c.connectHandshake(netConn, header, websocketKey); err != nil {
		return &Conn{conn: netConn}, err
	}

	return serveWebSocket(c, request, netConn, brw, c.EventHandler, compressEnabled), nil
}
