package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
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

func (c *Upgrader) connectHandshake(r *http.Request, responseHeader http.Header, conn net.Conn, websocketKey string) error {
	if r.Header.Get(internal.SecWebSocketProtocol.Key) != "" {
		var subprotocolsUsed = ""
		var arr = internal.Split(r.Header.Get(internal.SecWebSocketProtocol.Key), ",")
		for _, item := range arr {
			if internal.InCollection(item, c.option.Subprotocols) {
				subprotocolsUsed = item
				break
			}
		}
		if subprotocolsUsed != "" {
			responseHeader.Set(internal.SecWebSocketProtocol.Key, subprotocolsUsed)
		}
	}

	var buf = make([]byte, 0, 256)
	buf = append(buf, "HTTP/1.1 101 Switching Protocols\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Sec-WebSocket-Accept: "...)
	buf = append(buf, internal.ComputeAcceptKey(websocketKey)...)
	buf = append(buf, "\r\n"...)
	for k, _ := range responseHeader {
		buf = append(buf, k...)
		buf = append(buf, ": "...)
		buf = append(buf, responseHeader.Get(k)...)
		buf = append(buf, "\r\n"...)
	}
	buf = append(buf, "\r\n"...)
	_, err := conn.Write(buf)
	return err
}

// Accept http upgrade to websocket protocol
func (c *Upgrader) Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	netConn, br, err := c.hijack(w)
	if err != nil {
		return nil, err
	}

	socket, err := c.doAccept(r, netConn, br)
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}
	return socket, err
}

func (c *Upgrader) hijack(w http.ResponseWriter) (net.Conn, *bufio.Reader, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, internal.CloseInternalServerErr
	}
	netConn, brw, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}

	brw.Writer = nil
	if brw.Reader.Size() != c.option.ReadBufferSize {
		brw.Reader = bufio.NewReaderSize(netConn, c.option.ReadBufferSize)
	}
	return netConn, brw.Reader, nil
}

func (c *Upgrader) doAccept(r *http.Request, netConn net.Conn, br *bufio.Reader) (*Conn, error) {
	var session = new(sliceMap)
	var header = c.option.ResponseHeader.Clone()
	if !c.option.CheckOrigin(r, session) {
		return nil, internal.ErrCheckOrigin
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, internal.ErrGetMethodRequired
	}
	if !internal.HttpHeaderEqual(r.Header.Get(internal.SecWebSocketVersion.Key), internal.SecWebSocketVersion.Val) {
		msg := "websocket version not supported"
		return nil, errors.New(msg)
	}
	if !internal.HttpHeaderEqual(r.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return nil, internal.ErrHandshake
	}
	if !internal.HttpHeaderEqual(r.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
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

	if err := c.connectHandshake(r, header, netConn, websocketKey); err != nil {
		return &Conn{conn: netConn}, err
	}
	if err := netConn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if err := setNoDelay(netConn); err != nil {
		return nil, err
	}
	return serveWebSocket(true, c.option.getConfig(), session, netConn, br, c.eventHandler, compressEnabled), nil
}

type Server struct {
	upgrader *Upgrader
	OnError  func(conn net.Conn, err error)
}

func NewServer(eventHandler Event, option *ServerOption) (*Server, error) {
	var c = &Server{upgrader: NewUpgrader(eventHandler, option)}
	c.OnError = func(conn net.Conn, err error) {}
	return c, nil
}

func (c *Server) parseRequest(br *bufio.Reader) (*http.Request, error) {
	index := 0
	request := &http.Request{Header: http.Header{}}
	for {
		index++
		line, isPrefix, err := br.ReadLine()
		if isPrefix {
			return nil, internal.ErrLongLine
		}
		if err != nil {
			return nil, err
		}

		if len(line) == 0 {
			break
		}
		if index == 1 {
			arr := bytes.Split(line, []byte(" "))
			if len(arr) == 3 {
				request.Method = string(arr[0])
				URL, err := url.Parse(string(arr[1]))
				if err != nil {
					return nil, err
				}
				request.URL = URL
				request.Proto = string(arr[2])
			} else {
				return nil, internal.ErrHandshake
			}
			continue
		}

		arr := strings.Split(string(line), ": ")
		if len(arr) != 2 {
			return nil, internal.ErrHandshake
		}
		request.Header.Set(arr[0], arr[1])
	}
	return request, nil
}

func (c *Server) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.serve(listener)
}

func (c *Server) RunTLS(addr string, config *tls.Config) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.serve(tls.NewListener(listener, config))
}

func (c *Server) serve(listener net.Listener) error {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			c.OnError(conn, err)
			continue
		}

		go func() {
			br := bufio.NewReaderSize(conn, c.upgrader.option.ReadBufferSize)
			r, err := c.parseRequest(br)
			if err != nil {
				c.OnError(conn, err)
				return
			}

			socket, err := c.upgrader.doAccept(r, conn, br)
			if err != nil {
				_ = conn.Close()
				c.OnError(conn, err)
				return
			}
			socket.Listen()
		}()
	}
}
