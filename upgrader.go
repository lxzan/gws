package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lxzan/gws/internal"
)

type responseWriter struct {
	err         error
	b           *bytes.Buffer
	subprotocol string
}

func (c *responseWriter) Init() *responseWriter {
	c.b = binaryPool.Get(512)
	c.b.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	c.b.WriteString("Upgrade: websocket\r\n")
	c.b.WriteString("Connection: Upgrade\r\n")
	return c
}

func (c *responseWriter) Close() {
	binaryPool.Put(c.b)
	c.b = nil
}

func (c *responseWriter) WithHeader(k, v string) {
	c.b.WriteString(k)
	c.b.WriteString(": ")
	c.b.WriteString(v)
	c.b.WriteString("\r\n")
}

func (c *responseWriter) WithExtraHeader(h http.Header) {
	for k, _ := range h {
		c.WithHeader(k, h.Get(k))
	}
}

func (c *responseWriter) WithSubProtocol(requestHeader http.Header, expectedSubProtocols []string) {
	if len(expectedSubProtocols) > 0 {
		c.subprotocol = internal.GetIntersectionElem(expectedSubProtocols, internal.Split(requestHeader.Get(internal.SecWebSocketProtocol.Key), ","))
		if c.subprotocol == "" {
			c.err = ErrSubprotocolNegotiation
			return
		}
		c.WithHeader(internal.SecWebSocketProtocol.Key, c.subprotocol)
	}
}

func (c *responseWriter) Write(conn net.Conn, timeout time.Duration) error {
	if c.err != nil {
		return c.err
	}
	c.b.WriteString("\r\n")
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if _, err := c.b.WriteTo(conn); err != nil {
		return err
	}
	return conn.SetDeadline(time.Time{})
}

type Upgrader struct {
	option       *ServerOption
	deflaterPool *deflaterPool
	eventHandler Event
}

func NewUpgrader(eventHandler Event, option *ServerOption) *Upgrader {
	u := &Upgrader{
		option:       initServerOption(option),
		eventHandler: eventHandler,
	}
	if u.option.PermessageDeflate.Enabled {
		u.deflaterPool = new(deflaterPool).initialize(u.option.PermessageDeflate)
	}
	return u
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

// 为了节省内存, 不复用hijack返回的bufio.ReadWriter
func (c *Upgrader) hijack(w http.ResponseWriter) (net.Conn, *bufio.Reader, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, internal.CloseInternalServerErr
	}
	netConn, _, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}
	br := c.option.config.readerPool.Get()
	br.Reset(netConn)
	return netConn, br, nil
}

func (c *Upgrader) doUpgrade(r *http.Request, netConn net.Conn, br *bufio.Reader) (*Conn, error) {
	var session = c.option.NewSession()
	if !c.option.Authorize(r, session) {
		return nil, ErrUnauthorized
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, ErrHandshake
	}
	if !strings.EqualFold(r.Header.Get(internal.SecWebSocketVersion.Key), internal.SecWebSocketVersion.Val) {
		return nil, errors.New("gws: websocket version not supported")
	}
	if !internal.HttpHeaderContains(r.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return nil, ErrHandshake
	}
	if !strings.EqualFold(r.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return nil, ErrHandshake
	}

	var rw = new(responseWriter).Init()
	defer rw.Close()
	if val := r.Header.Get(internal.SecWebSocketExtensions.Key); strings.Contains(val, internal.PermessageDeflate) && c.option.PermessageDeflate.Enabled {
		rw.WithHeader(internal.SecWebSocketExtensions.Key, c.option.PermessageDeflate.genResponseHeader())
		compressEnabled = true
	}
	var websocketKey = r.Header.Get(internal.SecWebSocketKey.Key)
	if websocketKey == "" {
		return nil, ErrHandshake
	}
	rw.WithHeader(internal.SecWebSocketAccept.Key, internal.ComputeAcceptKey(websocketKey))
	rw.WithSubProtocol(r.Header, c.option.SubProtocols)
	rw.WithExtraHeader(c.option.ResponseHeader)
	if err := rw.Write(netConn, c.option.HandshakeTimeout); err != nil {
		return nil, err
	}

	socket := &Conn{
		ss:                session,
		isServer:          true,
		subprotocol:       rw.subprotocol,
		compressEnabled:   compressEnabled,
		conn:              netConn,
		config:            c.option.getConfig(),
		br:                br,
		continuationFrame: continuationFrame{},
		fh:                frameHeader{},
		handler:           c.eventHandler,
		closed:            0,
	}
	if compressEnabled {
		socket.deflater = c.deflaterPool.Select()
	}
	return socket.init(), nil
}

type Server struct {
	upgrader *Upgrader
	option   *ServerOption

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
	c.option = c.upgrader.option
	c.OnError = func(conn net.Conn, err error) { c.option.Logger.Error("gws: " + err.Error()) }
	c.OnRequest = func(socket *Conn, request *http.Request) { socket.ReadLoop() }
	return c
}

// Run 运行. 可以被多次调用, 监听不同的地址.
// It can be called multiple times, listening to different addresses.
func (c *Server) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.RunListener(listener)
}

// RunTLS 运行. 可以被多次调用, 监听不同的地址.
// It can be called multiple times, listening to different addresses.
func (c *Server) RunTLS(addr string, certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	if c.option.TlsConfig == nil {
		c.option.TlsConfig = &tls.Config{}
	}
	config := c.option.TlsConfig.Clone()
	config.Certificates = []tls.Certificate{cert}
	config.NextProtos = []string{"http/1.1"}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.RunListener(tls.NewListener(listener, config))
}

// RunListener 运行网络监听器
// Running the network listener
func (c *Server) RunListener(listener net.Listener) error {
	defer listener.Close()

	for {
		netConn, err := listener.Accept()
		if err != nil {
			c.OnError(netConn, err)
			continue
		}

		go func(conn net.Conn) {
			br := c.option.config.readerPool.Get()
			br.Reset(conn)
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
