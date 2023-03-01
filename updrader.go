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
	defaultAsyncReadGoLimit     = 8
	defaultAsyncWriteCap        = 128
	defaultCompressLevel        = flate.BestSpeed
	defaultMaxContentLength     = 16 * 1024 * 1024 // 16MiB
	defaultCompressionThreshold = 512              // 512 Byte
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

		// goroutine limits on concurrent read
		AsyncReadGoLimit int

		// capacity of async write queue
		// if the capacity is full, the message will be discarded
		AsyncWriteCap int

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
	if c.AsyncReadGoLimit <= 0 {
		c.AsyncReadGoLimit = defaultAsyncReadGoLimit
	}
	if c.AsyncWriteCap <= 0 {
		c.AsyncWriteCap = defaultAsyncWriteCap
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
	var request = &Request{Request: r, SessionStorage: &sliceMap{}}
	var header = internal.CloneHeader(c.ResponseHeader)
	if !c.CheckOrigin(request) {
		return nil, internal.ErrCheckOrigin
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, internal.ErrGetMethodRequired
	}
	if version := r.Header.Get(internal.SecWebSocketVersion.Key); version != internal.SecWebSocketVersion.Val {
		msg := "websocket protocol not supported: " + version
		return nil, errors.New(msg)
	}
	if val := r.Header.Get(internal.Connection.Key); strings.ToLower(val) != strings.ToLower(internal.Connection.Val) {
		return nil, internal.ErrHandshake
	}
	if val := r.Header.Get(internal.Upgrade.Key); strings.ToLower(val) != internal.Upgrade.Val {
		return nil, internal.ErrHandshake
	}
	if val := r.Header.Get(internal.SecWebSocketExtensions.Key); strings.Contains(val, "permessage-deflate") && c.CompressEnabled {
		header.Set(internal.SecWebSocketExtensions.Key, internal.SecWebSocketExtensions.Val)
		compressEnabled = true
	}
	var websocketKey = r.Header.Get(internal.SecWebSocketKey.Key)
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

	if err := internal.Errors(func() error {
		return netConn.SetDeadline(time.Time{})
	}, func() error {
		return netConn.SetReadDeadline(time.Time{})
	}, func() error {
		return netConn.SetWriteDeadline(time.Time{})
	}, func() error {
		return setNoDelay(netConn)
	}); err != nil {
		return nil, err
	}
	return serveWebSocket(c, request, netConn, brw, c.EventHandler, compressEnabled), nil
}
