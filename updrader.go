package websocket

import (
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	serverSide = 0
	clientSide = 1
)

type (
	Upgrader struct {
		*ServerOptions                       // config
		Header         http.Header           // set response header
		middlewares    []HandlerFunc         // middlewares
		CheckOrigin    func(r *Request) bool // filter user request
	}

	Request struct {
		*http.Request               // http request
		Storage       *internal.Map // store user session
	}
)

// use middleware
func (c *Upgrader) Use(handlers ...HandlerFunc) {
	c.middlewares = append(c.middlewares, handlers...)
}

func (c *Upgrader) handshake(conn net.Conn, websocketKey string) error {
	var buf = make([]byte, 0, 256)
	buf = append(buf, "HTTP/1.1 101 Switching Protocols\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Sec-WebSocket-Accept: "...)
	buf = append(buf, internal.ComputeAcceptKey(websocketKey)...)
	buf = append(buf, "\r\n"...)
	for k, _ := range c.Header {
		buf = append(buf, k...)
		buf = append(buf, ": "...)
		buf = append(buf, c.Header.Get(k)...)
		buf = append(buf, "\r\n"...)
	}
	buf = append(buf, "\r\n"...)
	_, err := conn.Write(buf)
	return err
}

// http protocol upgrade to websocket
func (c *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, handler EventHandler) error {
	if c.ServerOptions == nil {
		var options = defaultConfig
		c.ServerOptions = &options
	}
	c.ServerOptions.init()

	var request = &Request{Request: r, Storage: internal.NewMap()}
	if c.Header == nil {
		c.Header = http.Header{}
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return errors.New("http method must be get")
	}
	if version := r.Header.Get(internal.SecWebSocketVersion); version != internal.SecWebSocketVersion_Value {
		msg := "websocket protocol not supported: " + version
		return errors.New(msg)
	}
	if val := r.Header.Get(internal.Connection); strings.ToLower(val) != strings.ToLower(internal.Connection_Value) {
		return ERR_WebSocketHandshake
	}
	if val := r.Header.Get(internal.Upgrade); strings.ToLower(val) != internal.Upgrade_Value {
		return ERR_WebSocketHandshake
	}
	if val := r.Header.Get(internal.SecWebSocketExtensions); strings.Contains(val, "permessage-deflate") && c.CompressEnabled {
		c.Header.Set(internal.SecWebSocketExtensions, "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
		compressEnabled = true
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return CloseInternalServerErr
	}
	netConn, _, err := hj.Hijack()
	if err != nil {
		return err
	}
	if !c.CheckOrigin(request) {
		return ERR_CheckOrigin
	}

	// handshake with timeout control
	if err := netConn.SetDeadline(time.Now().Add(c.HandshakeTimeout)); err != nil {
		return err
	}
	var websocketKey = r.Header.Get(internal.SecWebSocketKey)
	if err := c.handshake(netConn, websocketKey); err != nil {
		return err
	}
	if err := netConn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	if err := netConn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}

	serveWebSocket(c, request, netConn, compressEnabled, handler)
	return nil
}
