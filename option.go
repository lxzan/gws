package gws

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

const (
	// 默认的并行协程限制
	// Default parallel goroutine limit
	defaultParallelGolimit = 8

	// 默认的压缩级别
	// Default compression level
	defaultCompressLevel = flate.BestSpeed

	// 默认的读取最大负载大小
	// Default maximum payload size for reading
	defaultReadMaxPayloadSize = 16 * 1024 * 1024

	// 默认的写入最大负载大小
	// Default maximum payload size for writing
	defaultWriteMaxPayloadSize = 16 * 1024 * 1024

	// 默认的压缩阈值
	// Default compression threshold
	defaultCompressThreshold = 512

	// 默认的压缩器池大小
	// Default compressor pool size
	defaultCompressorPoolSize = 32

	// 默认的读取缓冲区大小
	// Default read buffer size
	defaultReadBufferSize = 4 * 1024

	// 默认的写入缓冲区大小
	// Default write buffer size
	defaultWriteBufferSize = 4 * 1024

	// 默认的握手超时时间
	// Default handshake timeout
	defaultHandshakeTimeout = 5 * time.Second

	// 默认的拨号超时时间
	// Default dial timeout
	defaultDialTimeout = 5 * time.Second
)

type (
	// PermessageDeflate 压缩拓展配置
	// 对于gws client, 建议开启上下文接管, 不修改滑动窗口指数, 提供最好的兼容性.
	// 对于gws server, 如果开启上下文接管, 每个连接会占用更多内存, 合理配置滑动窗口指数.
	// For gws client, it is recommended to enable contextual takeover and not modify the sliding window index to provide the best compatibility.
	// For gws server, if you turn on context-side takeover, each connection takes up more memory, configure the sliding window index appropriately.
	PermessageDeflate struct {
		// 是否开启压缩
		// Whether to turn on compression
		Enabled bool

		// 压缩级别
		// Compress level
		Level int

		// 压缩阈值, 长度小于阈值的消息不会被压缩
		// Compression threshold, messages below the threshold will not be compressed
		Threshold int

		// 压缩器内存池大小
		// 数值越大竞争的概率越小, 但是会耗费大量内存
		// Compressor memory pool size
		// The higher the value the lower the probability of competition, but it will consume a lot of memory
		PoolSize int

		// 服务端上下文接管
		// Server side context takeover
		ServerContextTakeover bool

		// 客户端上下文接管
		// Client side context takeover
		ClientContextTakeover bool

		// 服务端滑动窗口指数
		// 取值范围 8<=n<=15, 表示pow(2,n)个字节
		// The server-side sliding window index
		// Range 8<=n<=15, means pow(2,n) bytes.
		ServerMaxWindowBits int

		// 客户端滑动窗口指数
		// 取值范围 8<=x<=15, 表示pow(2,n)个字节
		// The client-side sliding window index
		// Range 8<=n<=15, means pow(2,n) bytes.
		ClientMaxWindowBits int
	}

	Config struct {
		// bufio.Reader内存池
		brPool *internal.Pool[*bufio.Reader]

		// 压缩器滑动窗口内存池
		cswPool *internal.Pool[[]byte]

		// 解压器滑动窗口内存池
		dswPool *internal.Pool[[]byte]

		// 是否开启并行消息处理
		// Whether to enable parallel message processing
		ParallelEnabled bool

		// (单个连接)用于并行消息处理的协程数量限制
		// Limit on the number of concurrent goroutine used for parallel message processing (single connection)
		ParallelGolimit int

		// 最大读取的消息内容长度
		// Maximum read message content length
		ReadMaxPayloadSize int

		// 读缓冲区的大小
		// Size of the read buffer
		ReadBufferSize int

		// 最大写入的消息内容长度
		// Maximum length of written message content
		WriteMaxPayloadSize int

		// 写缓冲区的大小, v1.4.5版本此参数被废弃
		// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
		WriteBufferSize int

		// 是否检查文本utf8编码, 关闭性能会好点
		// Whether to check the text utf8 encoding, turn off the performance will be better
		CheckUtf8Enabled bool

		// 消息回调(OnMessage)的恢复程序
		// Message callback (OnMessage) recovery program
		Recovery func(logger Logger)

		// 日志工具
		// Logging tools
		Logger Logger
	}

	// ServerOption 结构体定义，用于配置 WebSocket 服务器的选项
	// ServerOption struct definition, used to configure WebSocket server options
	ServerOption struct {
		// 配置
		// Configuration
		config *Config

		// 写缓冲区的大小, v1.4.5版本此参数被废弃
		// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
		WriteBufferSize int

		// PermessageDeflate 配置
		// PermessageDeflate configuration
		PermessageDeflate PermessageDeflate

		// 是否启用并行处理
		// Whether parallel processing is enabled
		ParallelEnabled bool

		// 并行协程限制
		// Parallel goroutine limit
		ParallelGolimit int

		// 读取最大负载大小
		// Maximum payload size for reading
		ReadMaxPayloadSize int

		// 读取缓冲区大小
		// Read buffer size
		ReadBufferSize int

		// 写入最大负载大小
		// Maximum payload size for writing
		WriteMaxPayloadSize int

		// 是否启用 UTF-8 检查
		// Whether UTF-8 check is enabled
		CheckUtf8Enabled bool

		// 日志记录器
		// Logger
		Logger Logger

		// 恢复函数
		// Recovery function
		Recovery func(logger Logger)

		// TLS 设置
		// TLS configuration
		TlsConfig *tls.Config

		// 握手超时时间
		// Handshake timeout duration
		HandshakeTimeout time.Duration

		// WebSocket 子协议, 握手失败会断开连接
		// WebSocket sub-protocol, handshake failure disconnects the connection
		SubProtocols []string

		// 额外的响应头(可能不受客户端支持)
		// Additional response headers (may not be supported by the client)
		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		ResponseHeader http.Header

		// 鉴权函数，用于连接建立的请求
		// Authentication function for connection establishment requests
		Authorize func(r *http.Request, session SessionStorage) bool

		// 创建 session 存储空间，用于自定义 SessionStorage 实现
		// Create session storage space for custom SessionStorage implementations
		NewSession func() SessionStorage
	}
)

// 设置压缩阈值
// 开启上下文接管时, 必须不论长短压缩全部消息, 否则浏览器会报错
// when context takeover is enabled, all messages must be compressed regardless of length,
// otherwise the browser will report an error.
func (c *PermessageDeflate) setThreshold(isServer bool) {
	if (isServer && c.ServerContextTakeover) || (!isServer && c.ClientContextTakeover) {
		c.Threshold = 0
	}
}

// deleteProtectedHeaders 删除受保护的 WebSocket 头部字段
// deleteProtectedHeaders removes protected WebSocket header fields
func (c *ServerOption) deleteProtectedHeaders() {
	// 删除 Upgrade 头部字段
	// Remove the Upgrade header field
	c.ResponseHeader.Del(internal.Upgrade.Key)

	// 删除 Connection 头部字段
	// Remove the Connection header field
	c.ResponseHeader.Del(internal.Connection.Key)

	// 删除 Sec-WebSocket-Accept 头部字段
	// Remove the Sec-WebSocket-Accept header field
	c.ResponseHeader.Del(internal.SecWebSocketAccept.Key)

	// 删除 Sec-WebSocket-Extensions 头部字段
	// Remove the Sec-WebSocket-Extensions header field
	c.ResponseHeader.Del(internal.SecWebSocketExtensions.Key)

	// 删除 Sec-WebSocket-Protocol 头部字段
	// Remove the Sec-WebSocket-Protocol header field
	c.ResponseHeader.Del(internal.SecWebSocketProtocol.Key)
}

// 初始化服务器选项
// Initialize server options
func initServerOption(c *ServerOption) *ServerOption {
	// 如果 c 为 nil，则创建一个新的 ServerOption 实例
	// If c is nil, create a new ServerOption instance
	if c == nil {
		c = new(ServerOption)
	}

	// 如果 ReadMaxPayloadSize 小于等于 0，则设置为默认值
	// If ReadMaxPayloadSize is less than or equal to 0, set it to the default value
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}

	// 如果 ParallelGolimit 小于等于 0，则设置为默认值
	// If ParallelGolimit is less than or equal to 0, set it to the default value
	if c.ParallelGolimit <= 0 {
		c.ParallelGolimit = defaultParallelGolimit
	}

	// 如果 ReadBufferSize 小于等于 0，则设置为默认值
	// If ReadBufferSize is less than or equal to 0, set it to the default value
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}

	// 如果 WriteMaxPayloadSize 小于等于 0，则设置为默认值
	// If WriteMaxPayloadSize is less than or equal to 0, set it to the default value
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}

	// 如果 WriteBufferSize 小于等于 0，则设置为默认值
	// If WriteBufferSize is less than or equal to 0, set it to the default value
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}

	// 如果 Authorize 函数为 nil，则设置为默认函数
	// If the Authorize function is nil, set it to the default function
	if c.Authorize == nil {
		c.Authorize = func(r *http.Request, session SessionStorage) bool { return true }
	}

	// 如果 NewSession 函数为 nil，则设置为默认函数
	// If the NewSession function is nil, set it to the default function
	if c.NewSession == nil {
		c.NewSession = func() SessionStorage { return newSmap() }
	}

	// 如果 ResponseHeader 为 nil，则初始化为一个新的 http.Header
	// If ResponseHeader is nil, initialize it as a new http.Header
	if c.ResponseHeader == nil {
		c.ResponseHeader = http.Header{}
	}

	// 如果 HandshakeTimeout 小于等于 0，则设置为默认值
	// If HandshakeTimeout is less than or equal to 0, set it to the default value
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}

	// 如果 Logger 为 nil，则设置为默认日志记录器
	// If Logger is nil, set it to the default logger
	if c.Logger == nil {
		c.Logger = defaultLogger
	}

	// 如果 Recovery 函数为 nil，则设置为默认函数
	// If the Recovery function is nil, set it to the default function
	if c.Recovery == nil {
		c.Recovery = func(logger Logger) {}
	}

	// 如果启用了 PermessageDeflate，则进行相关配置
	// If PermessageDeflate is enabled, configure related settings
	if c.PermessageDeflate.Enabled {
		// 如果 ServerMaxWindowBits 不在 8 到 15 之间，则设置为默认值
		// If ServerMaxWindowBits is not between 8 and 15, set it to the default value
		if c.PermessageDeflate.ServerMaxWindowBits < 8 || c.PermessageDeflate.ServerMaxWindowBits > 15 {
			c.PermessageDeflate.ServerMaxWindowBits = internal.SelectValue(c.PermessageDeflate.ServerContextTakeover, 12, 15)
		}

		// 如果 ClientMaxWindowBits 不在 8 到 15 之间，则设置为默认值
		// If ClientMaxWindowBits is not between 8 and 15, set it to the default value
		if c.PermessageDeflate.ClientMaxWindowBits < 8 || c.PermessageDeflate.ClientMaxWindowBits > 15 {
			c.PermessageDeflate.ClientMaxWindowBits = internal.SelectValue(c.PermessageDeflate.ClientContextTakeover, 12, 15)
		}

		// 如果 Threshold 小于等于 0，则设置为默认值
		// If Threshold is less than or equal to 0, set it to the default value
		if c.PermessageDeflate.Threshold <= 0 {
			c.PermessageDeflate.Threshold = defaultCompressThreshold
		}

		// 如果 Level 等于 0，则设置为默认值
		// If Level is equal to 0, set it to the default value
		if c.PermessageDeflate.Level == 0 {
			c.PermessageDeflate.Level = defaultCompressLevel
		}

		// 如果 PoolSize 小于等于 0，则设置为默认值
		// If PoolSize is less than or equal to 0, set it to the default value
		if c.PermessageDeflate.PoolSize <= 0 {
			c.PermessageDeflate.PoolSize = defaultCompressorPoolSize
		}

		// 将 PoolSize 转换为二进制数
		// Convert PoolSize to a binary number
		c.PermessageDeflate.PoolSize = internal.ToBinaryNumber(c.PermessageDeflate.PoolSize)
	}

	// 删除受保护的头部信息
	// Delete protected headers
	c.deleteProtectedHeaders()

	// 配置 WebSocket 客户端的选项
	// Configure WebSocket client options
	c.config = &Config{
		// 是否启用并行处理
		// Whether parallel processing is enabled
		ParallelEnabled: c.ParallelEnabled,

		// 并行协程限制
		// Parallel goroutine limit
		ParallelGolimit: c.ParallelGolimit,

		// 读取最大负载大小
		// Maximum payload size for reading
		ReadMaxPayloadSize: c.ReadMaxPayloadSize,

		// 读取缓冲区大小
		// Read buffer size
		ReadBufferSize: c.ReadBufferSize,

		// 写入最大负载大小
		// Maximum payload size for writing
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,

		// 写缓冲区大小
		// Write buffer size
		WriteBufferSize: c.WriteBufferSize,

		// 是否启用 UTF-8 检查
		// Whether UTF-8 check is enabled
		CheckUtf8Enabled: c.CheckUtf8Enabled,

		// 恢复函数
		// Recovery function
		Recovery: c.Recovery,

		// 日志记录器
		// Logger
		Logger: c.Logger,

		// 缓冲区读取池
		// Buffer reader pool
		brPool: internal.NewPool(func() *bufio.Reader {
			return bufio.NewReaderSize(nil, c.ReadBufferSize)
		}),
	}

	// 如果启用了 PermessageDeflate，则进行相关配置
	// If PermessageDeflate is enabled, configure related settings
	if c.PermessageDeflate.Enabled {
		// 如果服务器上下文接管启用，则配置服务器窗口大小池
		// If server context takeover is enabled, configure server window size pool
		if c.PermessageDeflate.ServerContextTakeover {
			// 计算服务器窗口大小
			// Calculate server window size
			windowSize := internal.BinaryPow(c.PermessageDeflate.ServerMaxWindowBits)

			// 创建服务器窗口大小池
			// Create server window size pool
			c.config.cswPool = internal.NewPool[[]byte](func() []byte {
				return make([]byte, 0, windowSize)
			})
		}

		// 如果客户端上下文接管启用，则配置客户端窗口大小池
		// If client context takeover is enabled, configure client window size pool
		if c.PermessageDeflate.ClientContextTakeover {
			// 计算客户端窗口大小
			// Calculate client window size
			windowSize := internal.BinaryPow(c.PermessageDeflate.ClientMaxWindowBits)

			// 创建客户端窗口大小池
			// Create client window size pool
			c.config.dswPool = internal.NewPool[[]byte](func() []byte {
				return make([]byte, 0, windowSize)
			})
		}
	}

	// 返回配置后的客户端选项
	// Return the configured client options
	return c
}

// 获取通用配置
// Get common configuration
func (c *ServerOption) getConfig() *Config { return c.config }

// ClientOption 结构体定义，用于配置 WebSocket 客户端的选项
// ClientOption struct definition, used to configure WebSocket client options
type ClientOption struct {
	// 写缓冲区的大小, v1.4.5版本此参数被废弃
	// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
	WriteBufferSize int

	// PermessageDeflate 配置
	// PermessageDeflate configuration
	PermessageDeflate PermessageDeflate

	// 是否启用并行处理
	// Whether parallel processing is enabled
	ParallelEnabled bool

	// 并行协程限制
	// Parallel goroutine limit
	ParallelGolimit int

	// 读取最大负载大小
	// Maximum payload size for reading
	ReadMaxPayloadSize int

	// 读取缓冲区大小
	// Read buffer size
	ReadBufferSize int

	// 写入最大负载大小
	// Maximum payload size for writing
	WriteMaxPayloadSize int

	// 是否启用 UTF-8 检查
	// Whether UTF-8 check is enabled
	CheckUtf8Enabled bool

	// 日志记录器
	// Logger
	Logger Logger

	// 恢复函数
	// Recovery function
	Recovery func(logger Logger)

	// 连接地址, 例如 wss://example.com/connect
	// Server address, e.g., wss://example.com/connect
	Addr string

	// 额外的请求头
	// Extra request headers
	RequestHeader http.Header

	// 握手超时时间
	// Handshake timeout duration
	HandshakeTimeout time.Duration

	// TLS 设置
	// TLS configuration
	TlsConfig *tls.Config

	// 拨号器
	// 默认是返回 net.Dialer 实例, 也可以用于设置代理.
	// The default is to return the net.Dialer instance.
	// Can also be used to set a proxy, for example:
	// NewDialer: func() (proxy.Dialer, error) {
	//     return proxy.SOCKS5("tcp", "127.0.0.1:1080", nil, nil)
	// },
	NewDialer func() (Dialer, error)

	// 创建 session 存储空间
	// 用于自定义 SessionStorage 实现
	// For custom SessionStorage implementations
	NewSession func() SessionStorage
}

// 初始化客户端选项
// Initialize client options
func initClientOption(c *ClientOption) *ClientOption {
	// 如果 c 为 nil，则创建一个新的 ClientOption 实例
	// If c is nil, create a new ClientOption instance
	if c == nil {
		c = new(ClientOption)
	}

	// 如果 ReadMaxPayloadSize 小于等于 0，则设置为默认值
	// If ReadMaxPayloadSize is less than or equal to 0, set it to the default value
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}

	// 如果 ParallelGolimit 小于等于 0，则设置为默认值
	// If ParallelGolimit is less than or equal to 0, set it to the default value
	if c.ParallelGolimit <= 0 {
		c.ParallelGolimit = defaultParallelGolimit
	}

	// 如果 ReadBufferSize 小于等于 0，则设置为默认值
	// If ReadBufferSize is less than or equal to 0, set it to the default value
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}

	// 如果 WriteMaxPayloadSize 小于等于 0，则设置为默认值
	// If WriteMaxPayloadSize is less than or equal to 0, set it to the default value
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}

	// 如果 WriteBufferSize 小于等于 0，则设置为默认值
	// If WriteBufferSize is less than or equal to 0, set it to the default value
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}

	// 如果 HandshakeTimeout 小于等于 0，则设置为默认值
	// If HandshakeTimeout is less than or equal to 0, set it to the default value
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}

	// 如果 RequestHeader 为 nil，则初始化为一个新的 http.Header
	// If RequestHeader is nil, initialize it as a new http.Header
	if c.RequestHeader == nil {
		c.RequestHeader = http.Header{}
	}

	// 如果 NewDialer 函数为 nil，则设置为默认函数
	// If the NewDialer function is nil, set it to the default function
	if c.NewDialer == nil {
		c.NewDialer = func() (Dialer, error) { return &net.Dialer{Timeout: defaultDialTimeout}, nil }
	}

	// 如果 NewSession 函数为 nil，则设置为默认函数
	// If the NewSession function is nil, set it to the default function
	if c.NewSession == nil {
		c.NewSession = func() SessionStorage { return newSmap() }
	}

	// 如果 Logger 为 nil，则设置为默认日志记录器
	// If Logger is nil, set it to the default logger
	if c.Logger == nil {
		c.Logger = defaultLogger
	}

	// 如果 Recovery 函数为 nil，则设置为默认函数
	// If the Recovery function is nil, set it to the default function
	if c.Recovery == nil {
		c.Recovery = func(logger Logger) {}
	}

	// 如果启用了 PermessageDeflate，则进行相关配置
	// If PermessageDeflate is enabled, configure related settings
	if c.PermessageDeflate.Enabled {
		// 如果 ServerMaxWindowBits 不在 8 到 15 之间，则设置为默认值
		// If ServerMaxWindowBits is not between 8 and 15, set it to the default value
		if c.PermessageDeflate.ServerMaxWindowBits < 8 || c.PermessageDeflate.ServerMaxWindowBits > 15 {
			c.PermessageDeflate.ServerMaxWindowBits = 15
		}

		// 如果 ClientMaxWindowBits 不在 8 到 15 之间，则设置为默认值
		// If ClientMaxWindowBits is not between 8 and 15, set it to the default value
		if c.PermessageDeflate.ClientMaxWindowBits < 8 || c.PermessageDeflate.ClientMaxWindowBits > 15 {
			c.PermessageDeflate.ClientMaxWindowBits = 15
		}

		// 如果 Threshold 小于等于 0，则设置为默认值
		// If Threshold is less than or equal to 0, set it to the default value
		if c.PermessageDeflate.Threshold <= 0 {
			c.PermessageDeflate.Threshold = defaultCompressThreshold
		}

		// 如果 Level 等于 0，则设置为默认值
		// If Level is equal to 0, set it to the default value
		if c.PermessageDeflate.Level == 0 {
			c.PermessageDeflate.Level = defaultCompressLevel
		}

		// 设置 PoolSize 为 1
		// Set PoolSize to 1
		c.PermessageDeflate.PoolSize = 1
	}

	// 返回配置后的 ClientOption 实例
	// Return the configured ClientOption instance
	return c
}

// getConfig 方法将 ClientOption 的配置转换为 Config 并返回
// The getConfig method converts the ClientOption configuration to Config and returns it
func (c *ClientOption) getConfig() *Config {
	// 创建一个新的 Config 实例，并将 ClientOption 的各项配置赋值给它
	// Create a new Config instance and assign the ClientOption configurations to it
	config := &Config{
		// 并行处理是否启用
		// Whether parallel processing is enabled
		ParallelEnabled: c.ParallelEnabled,

		// 并行处理的协程限制
		// The goroutine limit for parallel processing
		ParallelGolimit: c.ParallelGolimit,

		// 读取的最大有效负载大小
		// The maximum payload size for reading
		ReadMaxPayloadSize: c.ReadMaxPayloadSize,

		// 读取缓冲区大小
		// The buffer size for reading
		ReadBufferSize: c.ReadBufferSize,

		// 写入的最大有效负载大小
		// The maximum payload size for writing
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,

		// 写入缓冲区大小
		// The buffer size for writing
		WriteBufferSize: c.WriteBufferSize,

		// 是否启用 UTF-8 检查
		// Whether UTF-8 checking is enabled
		CheckUtf8Enabled: c.CheckUtf8Enabled,

		// 恢复函数
		// The recovery function
		Recovery: c.Recovery,

		// 日志记录器
		// The logger
		Logger: c.Logger,
	}

	// 返回配置后的 Config 实例
	// Return the configured Config instance
	return config
}
