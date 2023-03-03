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
	defaultReadAsyncGoLimit    = 8
	defaultReadAsyncCap        = 256
	defaultWriteAsyncCap       = 256
	defaultCompressLevel       = flate.BestSpeed
	defaultReadMaxPayloadSize  = 16 * 1024 * 1024
	defaultWriteMaxPayloadSize = 16 * 1024 * 1024
	defaultCompressThreshold   = 512
)

type Upgrader struct {
	option       *ServerOption
	eventHandler Event
}

func NewUpgrader(eventHandler Event, option *ServerOption) *Upgrader {
	if option == nil {
		option = new(ServerOption)
	}
	return &Upgrader{
		option:       option.initialize(),
		eventHandler: eventHandler,
	}
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
	var session = new(sliceMap)
	var header = c.option.ResponseHeader.Clone()
	if !c.option.CheckOrigin(r, session) {
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
	if val := r.Header.Get(internal.SecWebSocketExtensions.Key); strings.Contains(val, "permessage-deflate") && c.option.CompressEnabled {
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

	if err := internal.Errors(
		func() error { return netConn.SetDeadline(time.Time{}) },
		func() error { return netConn.SetReadDeadline(time.Time{}) },
		func() error { return netConn.SetWriteDeadline(time.Time{}) },
		func() error { return setNoDelay(netConn) }); err != nil {
		return nil, err
	}
	return serveWebSocket(c.option.getConfig(), session, netConn, brw, c.eventHandler, compressEnabled), nil
}
