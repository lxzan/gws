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

// responseWriter 结构体定义
// responseWriter struct definition
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

// Init 方法初始化 responseWriter 结构体
// Init method initializes the responseWriter struct
func (c *responseWriter) Init() *responseWriter {
	// 从 binaryPool 获取一个大小为 512 的缓冲区
	// Get a buffer of size 512 from binaryPool
	c.b = binaryPool.Get(512)

	// 写入 HTTP 101 切换协议的响应头
	// Write the HTTP 101 Switching Protocols response header
	c.b.WriteString("HTTP/1.1 101 Switching Protocols\r\n")

	// 写入 Upgrade: websocket 头
	// Write the Upgrade: websocket header
	c.b.WriteString("Upgrade: websocket\r\n")

	// 写入 Connection: Upgrade 头
	// Write the Connection: Upgrade header
	c.b.WriteString("Connection: Upgrade\r\n")

	// 返回初始化后的 responseWriter 结构体
	// Return the initialized responseWriter struct
	return c
}

// Close 关闭 responseWriter 并将缓冲区放回池中
// Close closes the responseWriter and puts the buffer back into the pool
func (c *responseWriter) Close() {
	// 将缓冲区放回池中
	// Put the buffer back into the pool
	binaryPool.Put(c.b)

	// 将缓冲区指针置为 nil
	// Set the buffer pointer to nil
	c.b = nil
}

// WithHeader 向缓冲区中添加一个 HTTP 头部
// WithHeader adds an HTTP header to the buffer
func (c *responseWriter) WithHeader(k, v string) {
	// 写入头部键
	// Write the header key
	c.b.WriteString(k)

	// 写入冒号和空格
	// Write the colon and space
	c.b.WriteString(": ")

	// 写入头部值
	// Write the header value
	c.b.WriteString(v)

	// 写入回车换行符
	// Write the carriage return and newline characters
	c.b.WriteString("\r\n")
}

// WithExtraHeader 向缓冲区中添加多个 HTTP 头部
// WithExtraHeader adds multiple HTTP headers to the buffer
func (c *responseWriter) WithExtraHeader(h http.Header) {
	// 遍历所有头部
	// Iterate over all headers
	for k, _ := range h {
		// 添加每个头部键值对
		// Add each header key-value pair
		c.WithHeader(k, h.Get(k))
	}
}

// WithSubProtocol 根据请求头和预期的子协议列表设置子协议
// WithSubProtocol sets the subprotocol based on the request header and the expected subprotocols list
func (c *responseWriter) WithSubProtocol(requestHeader http.Header, expectedSubProtocols []string) {
	// 如果预期的子协议列表不为空
	// If the expected subprotocols list is not empty
	if len(expectedSubProtocols) > 0 {
		// 获取请求头中与预期子协议列表的交集元素
		// Get the intersection element from the request header and the expected subprotocols list
		c.subprotocol = internal.GetIntersectionElem(expectedSubProtocols, internal.Split(requestHeader.Get(internal.SecWebSocketProtocol.Key), ","))

		// 如果没有匹配的子协议
		// If there is no matching subprotocol
		if c.subprotocol == "" {
			// 设置错误为子协议协商失败
			// Set the error to subprotocol negotiation failure
			c.err = ErrSubprotocolNegotiation
			return
		}

		// 添加子协议头部
		// Add the subprotocol header
		c.WithHeader(internal.SecWebSocketProtocol.Key, c.subprotocol)
	}
}

// Write 将缓冲区内容写入连接，并设置超时
// Write writes the buffer content to the connection and sets the timeout
func (c *responseWriter) Write(conn net.Conn, timeout time.Duration) error {
	// 如果存在错误
	// If there is an error
	if c.err != nil {
		return c.err
	}

	// 在缓冲区末尾添加回车换行符
	// Add carriage return and newline characters at the end of the buffer
	c.b.WriteString("\r\n")

	// 设置连接的写入超时
	// Set the write timeout for the connection
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	// 将缓冲区内容写入连接
	// Write the buffer content to the connection
	if _, err := c.b.WriteTo(conn); err != nil {
		return err
	}

	// 重置连接的超时设置
	// Reset the timeout setting for the connection
	return conn.SetDeadline(time.Time{})
}

// Upgrader 结构体定义，用于处理 WebSocket 升级
// Upgrader struct definition, used for handling WebSocket upgrades
type Upgrader struct {
	// 服务器选项
	// Server options
	option *ServerOption

	// deflater 池
	// Deflater pool
	deflaterPool *deflaterPool

	// 事件处理器
	// Event handler
	eventHandler Event
}

// NewUpgrader 创建一个新的 Upgrader 实例
// NewUpgrader creates a new instance of Upgrader
func NewUpgrader(eventHandler Event, option *ServerOption) *Upgrader {
	// 初始化 Upgrader 实例
	// Initialize the Upgrader instance
	u := &Upgrader{
		// 初始化服务器选项
		// Initialize server options
		option: initServerOption(option),

		// 设置事件处理器
		// Set the event handler
		eventHandler: eventHandler,

		// 创建新的 deflater 池
		// Create a new deflater pool
		deflaterPool: new(deflaterPool),
	}

	// 如果启用了 PermessageDeflate
	// If PermessageDeflate is enabled
	if u.option.PermessageDeflate.Enabled {
		// 初始化 deflater 池
		// Initialize the deflater pool
		u.deflaterPool.initialize(u.option.PermessageDeflate, option.ReadMaxPayloadSize)
	}

	// 返回 Upgrader 实例
	// Return the Upgrader instance
	return u
}

// hijack 劫持 HTTP 连接并返回底层的网络连接和缓冲读取器
// hijack hijacks the HTTP connection and returns the underlying network connection and buffered reader
func (c *Upgrader) hijack(w http.ResponseWriter) (net.Conn, *bufio.Reader, error) {
	// 尝试将响应写入器转换为 Hijacker 接口
	// Attempt to cast the response writer to the Hijacker interface
	hj, ok := w.(http.Hijacker)

	// 如果转换失败，返回错误
	// If the cast fails, return an error
	if !ok {
		return nil, nil, internal.CloseInternalServerErr
	}

	// 劫持连接，获取底层网络连接
	// Hijack the connection to get the underlying network connection
	netConn, _, err := hj.Hijack()

	// 如果劫持失败，返回错误
	// If hijacking fails, return an error
	if err != nil {
		return nil, nil, err
	}

	// 从连接池中获取一个缓冲读取器
	// Get a buffered reader from the connection pool
	br := c.option.config.brPool.Get()

	// 重置缓冲读取器以使用新的网络连接
	// Reset the buffered reader to use the new network connection
	br.Reset(netConn)

	// 返回网络连接和缓冲读取器
	// Return the network connection and buffered reader
	return netConn, br, nil
}

// getPermessageDeflate 根据客户端和服务器的扩展协商结果获取 PermessageDeflate 配置
// getPermessageDeflate gets the PermessageDeflate configuration based on the negotiation results between the client and server extensions
func (c *Upgrader) getPermessageDeflate(extensions string) PermessageDeflate {
	// 从客户端扩展字符串中解析出客户端的 PermessageDeflate 配置
	// Parse the client's PermessageDeflate configuration from the extensions string
	clientPD := permessageNegotiation(extensions)

	// 获取服务器的 PermessageDeflate 配置
	// Get the server's PermessageDeflate configuration
	serverPD := c.option.PermessageDeflate

	// 初始化 PermessageDeflate 配置
	// Initialize the PermessageDeflate configuration
	pd := PermessageDeflate{
		// 启用状态取决于服务器是否启用并且扩展字符串中包含 PermessageDeflate
		// Enabled status depends on whether the server is enabled and the extensions string contains PermessageDeflate
		Enabled: serverPD.Enabled && strings.Contains(extensions, internal.PermessageDeflate),

		// 设置压缩阈值
		// Set the compression threshold
		Threshold: serverPD.Threshold,

		// 设置压缩级别
		// Set the compression level
		Level: serverPD.Level,

		// 设置池大小
		// Set the pool size
		PoolSize: serverPD.PoolSize,

		// 设置服务器上下文接管
		// Set the server context takeover
		ServerContextTakeover: clientPD.ServerContextTakeover && serverPD.ServerContextTakeover,

		// 设置客户端上下文接管
		// Set the client context takeover
		ClientContextTakeover: clientPD.ClientContextTakeover && serverPD.ClientContextTakeover,

		// 设置服务器最大窗口位
		// Set the server max window bits
		ServerMaxWindowBits: serverPD.ServerMaxWindowBits,

		// 设置客户端最大窗口位
		// Set the client max window bits
		ClientMaxWindowBits: serverPD.ClientMaxWindowBits,
	}

	// 设置压缩阈值
	// Set the compression threshold
	pd.setThreshold(true)

	// 返回 PermessageDeflate 配置
	// Return the PermessageDeflate configuration
	return pd
}

// Upgrade 升级 HTTP 连接到 WebSocket 连接
// Upgrade upgrades the HTTP connection to a WebSocket connection
func (c *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	// 劫持 HTTP 连接，获取底层网络连接和缓冲读取器
	// Hijack the HTTP connection to get the underlying network connection and buffered reader
	netConn, br, err := c.hijack(w)

	// 如果劫持失败，返回错误
	// If hijacking fails, return an error
	if err != nil {
		return nil, err
	}

	// 从网络连接升级到 WebSocket 连接
	// Upgrade from the network connection to a WebSocket connection
	return c.UpgradeFromConn(netConn, br, r)
}

// UpgradeFromConn 从现有的网络连接升级到 WebSocket 连接
// UpgradeFromConn upgrades from an existing network connection to a WebSocket connection
func (c *Upgrader) UpgradeFromConn(conn net.Conn, br *bufio.Reader, r *http.Request) (*Conn, error) {
	// 执行连接升级操作
	// Perform the connection upgrade operation
	socket, err := c.doUpgradeFromConn(conn, br, r)

	// 如果升级失败，写入错误信息并关闭连接
	// If the upgrade fails, write the error message and close the connection
	if err != nil {
		_ = c.writeErr(conn, err)
		_ = conn.Close()
	}

	// 返回 WebSocket 连接和错误信息
	// Return the WebSocket connection and error information
	return socket, err
}

// writeErr 向客户端写入 HTTP 错误响应
// writeErr writes an HTTP error response to the client
func (c *Upgrader) writeErr(conn net.Conn, err error) error {
	// 获取错误信息字符串
	// Get the error message string
	var str = err.Error()

	// 从缓冲池中获取一个缓冲区
	// Get a buffer from the buffer pool
	var buf = binaryPool.Get(256)

	// 写入 HTTP 状态行
	// Write the HTTP status line
	buf.WriteString("HTTP/1.1 400 Bad Request\r\n")

	// 写入当前日期
	// Write the current date
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123) + "\r\n")

	// 写入内容长度
	// Write the content length
	buf.WriteString("Content-Length: " + strconv.Itoa(len(str)) + "\r\n")

	// 写入内容类型
	// Write the content type
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")

	// 写入空行，表示头部结束
	// Write an empty line to indicate the end of the headers
	buf.WriteString("\r\n")

	// 写入错误信息
	// Write the error message
	buf.WriteString(str)

	// 将缓冲区内容写入连接
	// Write the buffer content to the connection
	_, result := buf.WriteTo(conn)

	// 将缓冲区放回缓冲池
	// Put the buffer back into the buffer pool
	binaryPool.Put(buf)

	// 返回写入结果
	// Return the write result
	return result
}

// doUpgradeFromConn 从现有的网络连接升级到 WebSocket 连接
// doUpgradeFromConn upgrades from an existing network connection to a WebSocket connection
func (c *Upgrader) doUpgradeFromConn(netConn net.Conn, br *bufio.Reader, r *http.Request) (*Conn, error) {
	// 创建一个新的会话
	// Create a new session
	var session = c.option.NewSession()

	// 授权请求，如果授权失败，返回未授权错误
	// Authorize the request, if authorization fails, return an unauthorized error
	if !c.option.Authorize(r, session) {
		return nil, ErrUnauthorized
	}

	// 检查请求方法是否为 GET，如果不是，返回握手错误
	// Check if the request method is GET, if not, return a handshake error
	if r.Method != http.MethodGet {
		return nil, ErrHandshake
	}

	// 检查 WebSocket 版本是否支持，如果不支持，返回错误
	// Check if the WebSocket version is supported, if not, return an error
	if !strings.EqualFold(r.Header.Get(internal.SecWebSocketVersion.Key), internal.SecWebSocketVersion.Val) {
		return nil, errors.New("gws: websocket version not supported")
	}

	// 检查 Connection 头是否包含正确的值，如果不包含，返回握手错误
	// Check if the Connection header contains the correct value, if not, return a handshake error
	if !internal.HttpHeaderContains(r.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return nil, ErrHandshake
	}

	// 检查 Upgrade 头是否包含正确的值，如果不包含，返回握手错误
	// Check if the Upgrade header contains the correct value, if not, return a handshake error
	if !strings.EqualFold(r.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return nil, ErrHandshake
	}

	// 初始化响应写入器
	// Initialize the response writer
	var rw = new(responseWriter).Init()
	defer rw.Close()

	// 获取扩展头
	// Get the extensions header
	var extensions = r.Header.Get(internal.SecWebSocketExtensions.Key)

	// 获取 PermessageDeflate 配置
	// Get the PermessageDeflate configuration
	var pd = c.getPermessageDeflate(extensions)

	// 如果启用了 PermessageDeflate，添加相应的响应头
	// If PermessageDeflate is enabled, add the corresponding response header
	if pd.Enabled {
		rw.WithHeader(internal.SecWebSocketExtensions.Key, pd.genResponseHeader())
	}

	// 获取 WebSocket 密钥
	// Get the WebSocket key
	var websocketKey = r.Header.Get(internal.SecWebSocketKey.Key)

	// 如果 WebSocket 密钥为空，返回握手错误
	// If the WebSocket key is empty, return a handshake error
	if websocketKey == "" {
		return nil, ErrHandshake
	}

	// 添加 Sec-WebSocket-Accept 头
	// Add the Sec-WebSocket-Accept header
	rw.WithHeader(internal.SecWebSocketAccept.Key, internal.ComputeAcceptKey(websocketKey))

	// 添加子协议头
	// Add the subprotocol header
	rw.WithSubProtocol(r.Header, c.option.SubProtocols)

	// 添加额外的响应头
	// Add extra response headers
	rw.WithExtraHeader(c.option.ResponseHeader)

	// 写入响应，如果失败，返回错误
	// Write the response, if it fails, return an error
	if err := rw.Write(netConn, c.option.HandshakeTimeout); err != nil {
		return nil, err
	}

	// 获取配置选项
	// Get configuration options
	config := c.option.getConfig()

	// 创建 WebSocket 连接实例
	// Create a WebSocket connection instance
	socket := &Conn{
		// 会话
		// Session
		ss: session,

		// 是否为服务器端
		// Is server side
		isServer: true,

		// 子协议
		// Subprotocol
		subprotocol: rw.subprotocol,

		// PermessageDeflate 配置
		// PermessageDeflate configuration
		pd: pd,

		// 网络连接
		// Network connection
		conn: netConn,

		// 配置
		// Configuration
		config: config,

		// 缓冲读取器
		// Buffered reader
		br: br,

		// 连续帧
		// Continuation frame
		continuationFrame: continuationFrame{},

		// 帧头
		// Frame header
		fh: frameHeader{},

		// 事件处理器
		// Event handler
		handler: c.eventHandler,

		// 关闭状态
		// Closed status
		closed: 0,

		// 写队列
		// Write queue
		writeQueue: workerQueue{maxConcurrency: 1},

		// 读队列
		// Read queue
		readQueue: make(channel, c.option.ParallelGolimit),
	}

	// 如果启用了 PermessageDeflate
	// If PermessageDeflate is enabled
	if pd.Enabled {
		// 选择 deflater
		// Select the deflater
		socket.deflater = c.deflaterPool.Select()

		// 如果服务器上下文接管启用
		// If server context takeover is enabled
		if c.option.PermessageDeflate.ServerContextTakeover {
			// 初始化服务器上下文窗口
			// Initialize the server context window
			socket.cpsWindow.initialize(config.cswPool, c.option.PermessageDeflate.ServerMaxWindowBits)
		}

		// 如果客户端上下文接管启用
		// If client context takeover is enabled
		if c.option.PermessageDeflate.ClientContextTakeover {
			// 初始化客户端上下文窗口
			// Initialize the client context window
			socket.dpsWindow.initialize(config.dswPool, c.option.PermessageDeflate.ClientMaxWindowBits)
		}
	}

	// 返回 WebSocket 连接
	// Return the WebSocket connection
	return socket, nil
}

// Server 结构体定义，用于处理 WebSocket 服务器的相关操作
// Server struct definition, used for handling WebSocket server-related operations
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
// NewServer creates a new WebSocket server instance
func NewServer(eventHandler Event, option *ServerOption) *Server {
	// 初始化服务器实例，并设置升级器
	// Initialize the server instance and set the upgrader
	var c = &Server{upgrader: NewUpgrader(eventHandler, option)}

	// 设置服务器选项配置
	// Set the server option configuration
	c.option = c.upgrader.option

	// 设置默认的错误处理回调函数
	// Set the default error handling callback function
	c.OnError = func(conn net.Conn, err error) {
		// 记录错误日志
		// Log the error
		c.option.Logger.Error("gws: " + err.Error())
	}

	// 设置默认的请求处理回调函数
	// Set the default request handling callback function
	c.OnRequest = func(conn net.Conn, br *bufio.Reader, r *http.Request) {
		// 尝试将 HTTP 连接升级到 WebSocket 连接
		// Attempt to upgrade the HTTP connection to a WebSocket connection
		socket, err := c.GetUpgrader().UpgradeFromConn(conn, br, r)

		// 如果升级失败，调用错误处理回调函数
		// If the upgrade fails, call the error handling callback function
		if err != nil {
			c.OnError(conn, err)
		} else {
			// 否则，启动 WebSocket 连接的读取循环
			// Otherwise, start the read loop for the WebSocket connection
			socket.ReadLoop()
		}
	}

	// 返回服务器实例
	// Return the server instance
	return c
}

// GetUpgrader 获取服务器的升级器实例
// GetUpgrader retrieves the upgrader instance of the server
func (c *Server) GetUpgrader() *Upgrader {
	return c.upgrader
}

// Run 启动 WebSocket 服务器，监听指定地址
// Run starts the WebSocket server and listens on the specified address
func (c *Server) Run(addr string) error {
	// 创建 TCP 监听器
	// Create a TCP listener
	listener, err := net.Listen("tcp", addr)

	// 如果监听失败，返回错误
	// If listening fails, return an error
	if err != nil {
		return err
	}

	// 使用监听器运行服务器
	// Run the server using the listener
	return c.RunListener(listener)
}

// RunTLS 启动支持 TLS 的 WebSocket 服务器，监听指定地址
// RunTLS starts the WebSocket server with TLS support and listens on the specified address
func (c *Server) RunTLS(addr string, certFile, keyFile string) error {
	// 加载 TLS 证书和私钥
	// Load the TLS certificate and private key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)

	// 如果加载失败，返回错误
	// If loading fails, return an error
	if err != nil {
		return err
	}

	// 如果服务器的 TLS 配置为空，初始化一个新的配置
	// If the server's TLS configuration is nil, initialize a new configuration
	if c.option.TlsConfig == nil {
		c.option.TlsConfig = &tls.Config{}
	}

	// 克隆服务器的 TLS 配置
	// Clone the server's TLS configuration
	config := c.option.TlsConfig.Clone()

	// 设置证书
	// Set the certificate
	config.Certificates = []tls.Certificate{cert}

	// 设置下一个协议为 HTTP/1.1
	// Set the next protocol to HTTP/1.1
	config.NextProtos = []string{"http/1.1"}

	// 创建 TCP 监听器
	// Create a TCP listener
	listener, err := net.Listen("tcp", addr)

	// 如果监听失败，返回错误
	// If listening fails, return an error
	if err != nil {
		return err
	}

	// 使用 TLS 监听器运行服务器
	// Run the server using the TLS listener
	return c.RunListener(tls.NewListener(listener, config))
}

// RunListener 使用指定的监听器运行 WebSocket 服务器
// RunListener runs the WebSocket server using the specified listener
func (c *Server) RunListener(listener net.Listener) error {
	// 确保在函数返回时关闭监听器
	// Ensure the listener is closed when the function returns
	defer listener.Close()

	// 无限循环，接受新的连接
	// Infinite loop to accept new connections
	for {
		// 接受新的网络连接
		// Accept a new network connection
		netConn, err := listener.Accept()

		// 如果接受连接时发生错误，调用错误处理回调函数并继续
		// If an error occurs while accepting the connection, call the error handling callback and continue
		if err != nil {
			c.OnError(netConn, err)
			continue
		}

		// 启动一个新的 goroutine 处理连接
		// Start a new goroutine to handle the connection
		go func(conn net.Conn) {
			// 从缓冲池中获取一个缓冲读取器
			// Get a buffered reader from the buffer pool
			br := c.option.config.brPool.Get()

			// 重置缓冲读取器以使用新的连接
			// Reset the buffered reader to use the new connection
			br.Reset(conn)

			// 尝试读取 HTTP 请求
			// Attempt to read the HTTP request
			if r, err := http.ReadRequest(br); err != nil {
				// 如果读取请求失败，调用错误处理回调函数
				// If reading the request fails, call the error handling callback
				c.OnError(conn, err)
			} else {
				// 如果读取请求成功，调用请求处理回调函数
				// If reading the request succeeds, call the request handling callback
				c.OnRequest(conn, br, r)
			}
		}(netConn)
	}
}
