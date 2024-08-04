package gws

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lxzan/gws/internal"
)

// Dialer 接口定义了拨号方法
// Dialer interface defines the dial method
type Dialer interface {
	// Dial 方法用于建立网络连接
	// Dial method is used to establish a network connection
	Dial(network, addr string) (c net.Conn, err error)
}

// connector 结构体用于管理客户端连接
// connector struct is used to manage client connections
type connector struct {
	// option 表示客户端选项
	// option represents client options
	option *ClientOption

	// conn 表示网络连接
	// conn represents the network connection
	conn net.Conn

	// eventHandler 表示事件处理器
	// eventHandler represents the event handler
	eventHandler Event

	// secWebsocketKey 表示 WebSocket 安全密钥
	// secWebsocketKey represents the WebSocket security key
	secWebsocketKey string
}

// NewClient 创建一个新的 WebSocket 客户端连接
// NewClient creates a new WebSocket client connection
func NewClient(handler Event, option *ClientOption) (*Conn, *http.Response, error) {
	// 初始化客户端选项
	// Initialize client options
	option = initClientOption(option)

	// 创建一个新的连接器实例
	// Create a new connector instance
	c := &connector{option: option, eventHandler: handler}

	// 解析 WebSocket 地址
	// Parse the WebSocket address
	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, nil, err
	}

	// 检查协议是否为 ws 或 wss
	// Check if the protocol is ws or wss
	if URL.Scheme != "ws" && URL.Scheme != "wss" {
		return nil, nil, ErrUnsupportedProtocol
	}

	// 判断是否启用 TLS
	// Determine if TLS is enabled
	var tlsEnabled = URL.Scheme == "wss"

	// 创建拨号器
	// Create a dialer
	dialer, err := option.NewDialer()
	if err != nil {
		return nil, nil, err
	}

	// 选择端口号，默认情况下 wss 使用 443 端口，ws 使用 80 端口
	// Select the port number, default to 443 for wss and 80 for ws
	port := internal.SelectValue(URL.Port() == "", internal.SelectValue(tlsEnabled, "443", "80"), URL.Port())

	// 选择主机名，默认情况下使用 127.0.0.1
	// Select the hostname, default to 127.0.0.1
	hp := internal.SelectValue(URL.Hostname() == "", "127.0.0.1", URL.Hostname()) + ":" + port

	// 通过拨号器拨号连接到服务器
	// Dial the server using the dialer
	c.conn, err = dialer.Dial("tcp", hp)
	if err != nil {
		return nil, nil, err
	}

	// 如果启用了 TLS，配置 TLS 设置
	// If TLS is enabled, configure TLS settings
	if tlsEnabled {
		// 如果没有提供 TlsConfig，则创建一个新的 tls.Config 实例
		// If TlsConfig is not provided, create a new tls.Config instance
		if option.TlsConfig == nil {
			option.TlsConfig = &tls.Config{}
		}

		// 如果 TlsConfig 中没有设置 ServerName，则使用 URL 的主机名
		// If ServerName is not set in TlsConfig, use the hostname from the URL
		if option.TlsConfig.ServerName == "" {
			option.TlsConfig.ServerName = URL.Hostname()
		}

		// 使用配置的 TlsConfig 创建一个新的 TLS 客户端连接
		// Create a new TLS client connection using the configured TlsConfig
		c.conn = tls.Client(c.conn, option.TlsConfig)
	}

	// 执行握手操作
	// Perform the handshake operation
	client, resp, err := c.handshake()
	if err != nil {
		_ = c.conn.Close()
	}

	// 返回客户端连接、HTTP 响应和错误信息
	// Return the client connection, HTTP response, and error information
	return client, resp, err
}

// NewClientFromConn 通过外部连接创建客户端, 支持 TCP/KCP/Unix Domain Socket
// Create New client via external connection, supports TCP/KCP/Unix Domain Socket.
func NewClientFromConn(handler Event, option *ClientOption, conn net.Conn) (*Conn, *http.Response, error) {
	// 初始化客户端选项
	// Initialize client options
	option = initClientOption(option)

	// 创建一个新的 connector 实例
	// Create a new connector instance
	c := &connector{option: option, conn: conn, eventHandler: handler}

	// 执行握手操作
	// Perform the handshake operation
	client, resp, err := c.handshake()

	// 如果握手失败，关闭连接
	// If the handshake fails, close the connection
	if err != nil {
		_ = c.conn.Close()
	}

	// 返回客户端连接、HTTP 响应和错误信息
	// Return the client connection, HTTP response, and error information
	return client, resp, err
}

// request 发送 HTTP 请求以发起 WebSocket 握手
// request sends an HTTP request to initiate a WebSocket handshake
func (c *connector) request() (*http.Response, *bufio.Reader, error) {
	// 设置连接的超时时间
	// Set the connection timeout
	_ = c.conn.SetDeadline(time.Now().Add(c.option.HandshakeTimeout))
	// 创建一个带有超时的上下文
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.option.HandshakeTimeout)
	defer cancel()

	// 创建一个新的 HTTP GET 请求
	// Create a new HTTP GET request
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.option.Addr, nil)
	if err != nil {
		return nil, nil, err
	}

	// 将客户端选项中的请求头复制到 HTTP 请求头中
	// Copy the request headers from client options to the HTTP request headers
	for k, v := range c.option.RequestHeader {
		r.Header[k] = v
	}

	// 设置 Connection 头为 "Upgrade"
	// Set the Connection header to "Upgrade"
	r.Header.Set(internal.Connection.Key, internal.Connection.Val)

	// 设置 Upgrade 头为 "websocket"
	// Set the Upgrade header to "websocket"
	r.Header.Set(internal.Upgrade.Key, internal.Upgrade.Val)

	// 设置 Sec-WebSocket-Version 头为 "13"
	// Set the Sec-WebSocket-Version header to "13"
	r.Header.Set(internal.SecWebSocketVersion.Key, internal.SecWebSocketVersion.Val)

	// 如果启用了每消息压缩扩展，则设置 Sec-WebSocket-Extensions 头
	// If per-message deflate extension is enabled, set the Sec-WebSocket-Extensions header
	if c.option.PermessageDeflate.Enabled {
		r.Header.Set(internal.SecWebSocketExtensions.Key, c.option.PermessageDeflate.genRequestHeader())
	}

	// 如果没有安全 WebSocket 密钥，则生成一个
	// Generate a security WebSocket key if not already set
	if c.secWebsocketKey == "" {
		// 创建一个 16 字节的数组用于存储密钥
		// Create a 16-byte array to store the key
		var key [16]byte

		// 使用内部方法生成前 8 字节的随机数并存储在 key 数组中
		// Use an internal method to generate a random number for the first 8 bytes and store it in the key array
		binary.BigEndian.PutUint64(key[0:8], internal.AlphabetNumeric.Uint64())

		// 使用内部方法生成后 8 字节的随机数并存储在 key 数组中
		// Use an internal method to generate a random number for the last 8 bytes and store it in the key array
		binary.BigEndian.PutUint64(key[8:16], internal.AlphabetNumeric.Uint64())

		// 将生成的密钥编码为 base64 字符串并赋值给 secWebsocketKey
		// Encode the generated key as a base64 string and assign it to secWebsocketKey
		c.secWebsocketKey = base64.StdEncoding.EncodeToString(key[0:])

		// 将生成的密钥设置到请求头中
		// Set the generated key in the request header
		r.Header.Set(internal.SecWebSocketKey.Key, c.secWebsocketKey)
	}

	// 创建一个用于接收错误的通道
	// Create a channel to receive errors
	var ch = make(chan error)

	// 启动一个 goroutine 发送请求
	// Start a goroutine to send the request
	go func() { ch <- r.Write(c.conn) }()

	// 等待请求完成或上下文超时
	// Wait for the request to complete or the context to timeout
	select {
	case err = <-ch:
		// 如果请求完成，将错误赋值给 err
		// If the request completes, assign the error to err
	case <-ctx.Done():
		// 如果上下文超时或取消，将上下文的错误赋值给 err
		// If the context times out or is canceled, assign the context's error to err
		err = ctx.Err()
	}

	// 如果发生错误，返回错误信息
	// If an error occurs, return the error information
	if err != nil {
		return nil, nil, err
	}

	// 创建一个带有指定缓冲区大小的 bufio.Reader
	// Create a bufio.Reader with the specified buffer size
	br := bufio.NewReaderSize(c.conn, c.option.ReadBufferSize)

	// 读取 HTTP 响应
	// Read the HTTP response
	resp, err := http.ReadResponse(br, r)

	// 返回 HTTP 响应、缓冲读取器和错误信息
	// Return the HTTP response, buffered reader, and error information
	return resp, br, err
}

// getPermessageDeflate 获取每消息压缩扩展的配置
// getPermessageDeflate retrieves the configuration for per-message deflate extension
func (c *connector) getPermessageDeflate(extensions string) PermessageDeflate {
	// 解析服务器端的每消息压缩扩展配置
	// Parse the server-side per-message deflate extension configuration
	serverPD := permessageNegotiation(extensions)

	// 获取客户端的每消息压缩配置
	// Get the client-side per-message deflate configuration
	clientPD := c.option.PermessageDeflate

	// 创建一个新的每消息压缩配置实例
	// Create a new instance of per-message deflate configuration
	pd := PermessageDeflate{
		// 启用状态取决于客户端配置和服务器扩展是否包含每消息压缩
		// Enabled status depends on client configuration and whether the server extensions include per-message deflate
		Enabled: clientPD.Enabled && strings.Contains(extensions, internal.PermessageDeflate),

		// 设置压缩阈值
		// Set the compression threshold
		Threshold: clientPD.Threshold,

		// 设置压缩级别
		// Set the compression level
		Level: clientPD.Level,

		// 设置缓冲池大小
		// Set the buffer pool size
		PoolSize: clientPD.PoolSize,

		// 设置服务器上下文接管配置
		// Set the server context takeover configuration
		ServerContextTakeover: serverPD.ServerContextTakeover,

		// 设置客户端上下文接管配置
		// Set the client context takeover configuration
		ClientContextTakeover: serverPD.ClientContextTakeover,

		// 设置服务器最大窗口位数
		// Set the server max window bits
		ServerMaxWindowBits: serverPD.ServerMaxWindowBits,

		// 设置客户端最大窗口位数
		// Set the client max window bits
		ClientMaxWindowBits: serverPD.ClientMaxWindowBits,
	}

	// 设置压缩阈值
	// Set the compression threshold
	pd.setThreshold(false)

	// 返回每消息压缩配置
	// Return the per-message deflate configuration
	return pd
}

// handshake 执行 WebSocket 握手操作
// handshake performs the WebSocket handshake operation
func (c *connector) handshake() (*Conn, *http.Response, error) {
	// 发送握手请求并读取响应
	// Send the handshake request and read the response
	resp, br, err := c.request()
	if err != nil {
		// 如果请求失败，返回错误信息
		// If the request fails, return the error information
		return nil, resp, err
	}

	// 检查响应头以验证握手是否成功
	// Check the response headers to verify if the handshake was successful
	if err = c.checkHeaders(resp); err != nil {
		// 如果握手失败，返回错误信息
		// If the handshake fails, return the error information
		return nil, resp, err
	}

	// 获取协商的子协议
	// Get the negotiated subprotocol
	subprotocol, err := c.getSubProtocol(resp)
	if err != nil {
		// 如果获取子协议失败，返回错误信息
		// If getting the subprotocol fails, return the error information
		return nil, resp, err
	}

	// 获取响应头中的扩展字段
	// Get the extensions field from the response header
	var extensions = resp.Header.Get(internal.SecWebSocketExtensions.Key)

	// 获取每消息压缩扩展的配置
	// Get the per-message deflate configuration
	var pd = c.getPermessageDeflate(extensions)

	// 创建 WebSocket 连接对象
	// Create the WebSocket connection object
	socket := &Conn{
		ss:                c.option.NewSession(),
		isServer:          false,
		subprotocol:       subprotocol,
		pd:                pd,
		conn:              c.conn,
		config:            c.option.getConfig(),
		br:                br,
		continuationFrame: continuationFrame{},
		fh:                frameHeader{},
		handler:           c.eventHandler,
		closed:            0,
		deflater:          new(deflater),
		writeQueue:        workerQueue{maxConcurrency: 1},
		readQueue:         make(channel, c.option.ParallelGolimit),
	}

	// 如果启用了每消息压缩扩展，初始化压缩器和窗口
	// If per-message deflate is enabled, initialize the deflater and windows
	if pd.Enabled {
		// 初始化压缩器，传入是否为服务器端、压缩配置和最大负载大小
		// Initialize the deflater, passing whether it is server-side, compression configuration, and max payload size
		socket.deflater.initialize(false, pd, c.option.ReadMaxPayloadSize)

		// 如果服务器上下文接管启用，初始化服务器端窗口
		// If server context takeover is enabled, initialize the server-side window
		if pd.ServerContextTakeover {
			socket.dpsWindow.initialize(nil, pd.ServerMaxWindowBits)
		}

		// 如果客户端上下文接管启用，初始化客户端窗口
		// If client context takeover is enabled, initialize the client-side window
		if pd.ClientContextTakeover {
			socket.cpsWindow.initialize(nil, pd.ClientMaxWindowBits)
		}
	}

	// 返回 WebSocket 连接对象、HTTP 响应和错误信息
	// Return the WebSocket connection object, HTTP response, and error information
	return socket, resp, c.conn.SetDeadline(time.Time{})
}

// getSubProtocol 从响应中获取子协议
// getSubProtocol retrieves the subprotocol from the response
func (c *connector) getSubProtocol(resp *http.Response) (string, error) {
	// 从请求头中获取客户端支持的子协议列表
	// Get the list of subprotocols supported by the client from the request header
	a := internal.Split(c.option.RequestHeader.Get(internal.SecWebSocketProtocol.Key), ",")

	// 从响应头中获取服务器支持的子协议列表
	// Get the list of subprotocols supported by the server from the response header
	b := internal.Split(resp.Header.Get(internal.SecWebSocketProtocol.Key), ",")

	// 获取客户端和服务器支持的子协议的交集
	// Get the intersection of subprotocols supported by both client and server
	subprotocol := internal.GetIntersectionElem(a, b)

	// 如果客户端支持子协议但未协商出共同的子协议，返回子协议协商错误
	// If the client supports subprotocols but no common subprotocol is negotiated, return subprotocol negotiation error
	if len(a) > 0 && subprotocol == "" {
		return "", ErrSubprotocolNegotiation
	}

	// 返回协商出的子协议
	// Return the negotiated subprotocol
	return subprotocol, nil
}

// checkHeaders 检查响应头以验证握手是否成功
// checkHeaders checks the response headers to verify if the handshake was successful
func (c *connector) checkHeaders(resp *http.Response) error {
	// 检查状态码是否为 101 Switching Protocols
	// Check if the status code is 101 Switching Protocols
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return ErrHandshake
	}

	// 检查响应头中的 Connection 字段是否包含 "Upgrade"
	// Check if the Connection field in the response header contains "Upgrade"
	if !internal.HttpHeaderContains(resp.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return ErrHandshake
	}

	// 检查响应头中的 Upgrade 字段是否为 "websocket"
	// Check if the Upgrade field in the response header is "websocket"
	if !strings.EqualFold(resp.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return ErrHandshake
	}

	// 检查 Sec-WebSocket-Accept 字段的值是否正确
	// Check if the Sec-WebSocket-Accept field value is correct
	if resp.Header.Get(internal.SecWebSocketAccept.Key) != internal.ComputeAcceptKey(c.secWebsocketKey) {
		return ErrHandshake
	}

	// 如果所有检查都通过，返回 nil 表示成功
	// If all checks pass, return nil to indicate success
	return nil
}
