package gws

import (
	"bufio"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lxzan/gws/internal"
)

type Upgrader struct {
	option       *ServerOption
	eventHandler Event
}

func NewUpgrader(eventHandler Event, option *ServerOption) *Upgrader {
	return &Upgrader{
		option:       initServerOption(option),
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

// Upgrade http upgrade to websocket protocol
func (c *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	netConn, br, err := c.hijack(w)
	if err != nil {
		return nil, err
	}

	socket, err := c.doUpgrade(r, netConn, br)
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

func (c *Upgrader) doUpgrade(r *http.Request, netConn net.Conn, br *bufio.Reader) (*Conn, error) {
	if err := netConn.SetDeadline(time.Now().Add(c.option.HandshakeTimeout)); err != nil {
		return nil, err
	}

	var session = c.option.NewSessionStorage()
	var header = c.option.ResponseHeader.Clone()
	if !c.option.Authorize(r, session) {
		return nil, ErrUnauthorized
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, ErrHandshake
	}
	if !strings.EqualFold(r.Header.Get(internal.SecWebSocketVersion.Key), internal.SecWebSocketVersion.Val) {
		msg := "websocket version not supported"
		return nil, errors.New(msg)
	}
	if !internal.HttpHeaderContains(r.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return nil, ErrHandshake
	}
	if !strings.EqualFold(r.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return nil, ErrHandshake
	}
	if val := r.Header.Get(internal.SecWebSocketExtensions.Key); strings.Contains(val, "permessage-deflate") && c.option.CompressEnabled {
		header.Set(internal.SecWebSocketExtensions.Key, internal.SecWebSocketExtensions.Val)
		compressEnabled = true
	}
	var websocketKey = r.Header.Get(internal.SecWebSocketKey.Key)
	if websocketKey == "" {
		return nil, ErrHandshake
	}

	if err := c.connectHandshake(r, header, netConn, websocketKey); err != nil {
		return nil, err
	}
	if err := netConn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return serveWebSocket(true, c.option.getConfig(), session, netConn, br, c.eventHandler, compressEnabled), nil
}

type Server struct {
	upgrader *Upgrader

	// OnError 接收握手过程中产生的错误回调
	// Receive error callbacks generated during the handshake
	OnError func(conn net.Conn, err error)

	// OnRequest
	OnRequest func(socket *Conn, request *http.Request)
}

// NewServer 创建websocket服务器
// create a websocket server
func NewServer(eventHandler Event, option *ServerOption) *Server {
	var c = &Server{upgrader: NewUpgrader(eventHandler, option)}
	c.OnError = func(conn net.Conn, err error) { log.Println("gws: " + err.Error()) }
	c.OnRequest = func(socket *Conn, request *http.Request) { socket.ReadLoop() }
	return c
}

// Run runs ws server
// addr: Address of the listener
func (c *Server) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.RunListener(listener)
}

// RunTLS runs wss server
// addr: Address of the listener
// config: tls config
func (c *Server) RunTLS(addr string, certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"http/1.1"}}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.RunListener(tls.NewListener(listener, config))
}

func (c *Server) RunListener(listener net.Listener) error {
	defer listener.Close()

	for {
		netConn, err := listener.Accept()
		if err != nil {
			c.OnError(netConn, err)
			continue
		}

		go func(conn net.Conn) {
			br := bufio.NewReaderSize(conn, c.upgrader.option.ReadBufferSize)
			r, err := http.ReadRequest(br)
			if err != nil {
				c.OnError(conn, err)
				_ = conn.Close()
				return
			}

			socket, err := c.upgrader.doUpgrade(r, conn, br)
			if err != nil {
				c.OnError(conn, err)
				_ = conn.Close()
				return
			}
			c.OnRequest(socket, r)
		}(netConn)
	}
}
