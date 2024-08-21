package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lxzan/gws/internal"
)

type responseWriter struct {
	// 错误信息
	// Error information
	err error

	// 字节缓冲区
	// Byte buffer
	b *bytes.Buffer

	// 子协议
	// Subprotocol
	subprotocol string
}

// Init 初始化
// Initializes the responseWriter struct
func (c *responseWriter) Init() *responseWriter {
	c.b = binaryPool.Get(512)
	c.b.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	c.b.WriteString("Upgrade: websocket\r\n")
	c.b.WriteString("Connection: Upgrade\r\n")
	return c
}

// Close 回收资源
// Recycling resources
func (c *responseWriter) Close() {
	binaryPool.Put(c.b)
	c.b = nil
}

// WithHeader 添加 HTTP Header
// Adds an http header
func (c *responseWriter) WithHeader(k, v string) {
	c.b.WriteString(k)
	c.b.WriteString(": ")
	c.b.WriteString(v)
	c.b.WriteString("\r\n")
}

// WithExtraHeader 添加额外的 HTTP Header
// Adds extra http header
func (c *responseWriter) WithExtraHeader(h http.Header) {
	for k, _ := range h {
		c.WithHeader(k, h.Get(k))
	}
}

// WithSubProtocol 根据请求头和预期的子协议列表设置子协议
// Sets the subprotocol based on the request header and the expected subprotocols list
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

// Write 将缓冲区内容写入连接，并设置超时
// Writes the buffer content to the connection and sets the timeout
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

// NewUpgrader 创建一个新的 Upgrader 实例
// Creates a new instance of Upgrader
func NewUpgrader(eventHandler Event, option *ServerOption) *Upgrader {
	u := &Upgrader{
		option:       initServerOption(option),
		eventHandler: eventHandler,
		deflaterPool: new(deflaterPool),
	}
	if u.option.PermessageDeflate.Enabled {
		u.deflaterPool.initialize(u.option.PermessageDeflate, option.ReadMaxPayloadSize)
	}
	return u
}

// 劫持 HTTP 连接并返回底层的网络连接和缓冲读取器
// Hijacks the HTTP connection and returns the underlying network connection and buffered reader
func (c *Upgrader) hijack(w http.ResponseWriter) (net.Conn, *bufio.Reader, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, internal.CloseInternalErr
	}
	netConn, _, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}
	br := c.option.config.brPool.Get()
	br.Reset(netConn)
	return netConn, br, nil
}

// 根据客户端和服务器的扩展协商结果获取 PermessageDeflate 配置
// Gets the PermessageDeflate configuration based on the negotiation results between the client and server extensions
func (c *Upgrader) getPermessageDeflate(extensions string) PermessageDeflate {
	clientPD := permessageNegotiation(extensions)
	serverPD := c.option.PermessageDeflate
	pd := PermessageDeflate{
		Enabled:               serverPD.Enabled && strings.Contains(extensions, internal.PermessageDeflate),
		Threshold:             serverPD.Threshold,
		Level:                 serverPD.Level,
		PoolSize:              serverPD.PoolSize,
		ServerContextTakeover: clientPD.ServerContextTakeover && serverPD.ServerContextTakeover,
		ClientContextTakeover: clientPD.ClientContextTakeover && serverPD.ClientContextTakeover,
		ServerMaxWindowBits:   serverPD.ServerMaxWindowBits,
		ClientMaxWindowBits:   serverPD.ClientMaxWindowBits,
	}
	pd.setThreshold(true)
	return pd
}

// Upgrade 升级 HTTP 连接到 WebSocket 连接
// Upgrades the HTTP connection to a WebSocket connection
func (c *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	netConn, br, err := c.hijack(w)
	if err != nil {
		return nil, err
	}
	return c.UpgradeFromConn(netConn, br, r)
}

// UpgradeFromConn 从现有的网络连接升级到 WebSocket 连接
// Upgrades from an existing network connection to a WebSocket connection
func (c *Upgrader) UpgradeFromConn(conn net.Conn, br *bufio.Reader, r *http.Request) (*Conn, error) {
	socket, err := c.doUpgradeFromConn(conn, br, r)
	if err != nil {
		_ = c.writeErr(conn, err)
		_ = conn.Close()
	}
	return socket, err
}

// 向客户端写入 HTTP 错误响应
// Writes an HTTP error response to the client
func (c *Upgrader) writeErr(conn net.Conn, err error) error {
	var str = err.Error()
	var buf = binaryPool.Get(256)
	buf.WriteString("HTTP/1.1 400 Bad Request\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123) + "\r\n")
	buf.WriteString("Content-Length: " + strconv.Itoa(len(str)) + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(str)
	_, result := buf.WriteTo(conn)
	binaryPool.Put(buf)
	return result
}

// 从现有的网络连接升级到 WebSocket 连接
// Upgrades from an existing network connection to a WebSocket connection
func (c *Upgrader) doUpgradeFromConn(netConn net.Conn, br *bufio.Reader, r *http.Request) (*Conn, error) {
	// 授权请求，如果授权失败，返回未授权错误
	// Authorize the request, if authorization fails, return an unauthorized error
	var session = c.option.NewSession()
	if !c.option.Authorize(r, session) {
		return nil, ErrUnauthorized
	}

	// 检查请求头
	// check request headers
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

	var extensions = r.Header.Get(internal.SecWebSocketExtensions.Key)
	var pd = c.getPermessageDeflate(extensions)
	if pd.Enabled {
		rw.WithHeader(internal.SecWebSocketExtensions.Key, pd.genResponseHeader())
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

	config := c.option.getConfig()
	socket := &Conn{
		ss:                session,
		isServer:          true,
		subprotocol:       rw.subprotocol,
		pd:                pd,
		conn:              netConn,
		config:            config,
		br:                br,
		continuationFrame: continuationFrame{},
		fh:                frameHeader{},
		handler:           c.eventHandler,
		closed:            0,
		writeQueue:        workerQueue{maxConcurrency: 1},
		readQueue:         make(channel, c.option.ParallelGolimit),
	}

	// 压缩字典和解压字典内存开销比较大, 故使用懒加载
	// Compressing and decompressing dictionaries has a large memory overhead, so use lazy loading.
	if pd.Enabled {
		socket.deflater = c.deflaterPool.Select()
		if pd.ServerContextTakeover {
			socket.cpsWindow.initialize(config.cswPool, pd.ServerMaxWindowBits)
		}
		if pd.ClientContextTakeover {
			socket.dpsWindow.initialize(config.dswPool, pd.ClientMaxWindowBits)
		}
	}
	return socket, nil
}

// Server WebSocket服务器
// Websocket server
type Server struct {
	// 升级器，用于将 HTTP 连接升级到 WebSocket 连接
	// Upgrader, used to upgrade HTTP connections to WebSocket connections
	upgrader *Upgrader

	// 服务器选项配置
	// Server option configuration
	option *ServerOption

	// 错误处理回调函数
	// Error handling callback function
	OnError func(conn net.Conn, err error)

	// 请求处理回调函数
	// Request handling callback function
	OnRequest func(conn net.Conn, br *bufio.Reader, r *http.Request)
}

// NewServer 创建一个新的 WebSocket 服务器实例
// Creates a new WebSocket server instance
func NewServer(eventHandler Event, option *ServerOption) *Server {
	var c = &Server{upgrader: NewUpgrader(eventHandler, option)}
	c.option = c.upgrader.option
	c.OnError = func(conn net.Conn, err error) { c.option.Logger.Error("gws: " + err.Error()) }
	c.OnRequest = func(conn net.Conn, br *bufio.Reader, r *http.Request) {
		socket, err := c.GetUpgrader().UpgradeFromConn(conn, br, r)
		if err != nil {
			c.OnError(conn, err)
		} else {
			socket.ReadLoop()
		}
	}
	return c
}

// GetUpgrader 获取服务器的升级器实例
// Retrieves the upgrader instance of the server
func (c *Server) GetUpgrader() *Upgrader {
	return c.upgrader
}

// Run 启动 WebSocket 服务器，监听指定地址
// Starts the WebSocket server and listens on the specified address
func (c *Server) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return c.RunListener(listener)
}

// RunTLS 启动支持 TLS 的 WebSocket 服务器，监听指定地址
// Starts the WebSocket server with TLS support and listens on the specified address
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

// RunListener 使用指定的监听器运行 WebSocket 服务器
// Runs the WebSocket server using the specified listener
func (c *Server) RunListener(listener net.Listener) error {
	defer listener.Close()

	for {
		netConn, err := listener.Accept()
		if err != nil {
			c.OnError(netConn, err)
			continue
		}

		go func(conn net.Conn) {
			br := c.option.config.brPool.Get()
			br.Reset(conn)
			if r, err := http.ReadRequest(br); err != nil {
				c.OnError(conn, err)
			} else {
				c.OnRequest(conn, br, r)
			}
		}(netConn)
	}
}
