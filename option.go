package gws

import (
	"compress/flate"
	"crypto/tls"
	"net/http"
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
	defaultReadBufferSize      = 4 * 1024
	defaultWriteBufferSize     = 4 * 1024

	defaultDialTimeout = 5 * time.Second
)

type (
	Config struct {
		// 是否开启异步读, 开启的话会并行调用OnMessage
		// Whether to enable asynchronous reading, if enabled OnMessage will be called in parallel
		ReadAsyncEnabled bool

		// 异步读的最大并行协程数量
		// Maximum number of parallel concurrent processes for asynchronous reads
		ReadAsyncGoLimit int

		// 异步读的容量限制, 容量溢出将会返回错误
		// Capacity limit for asynchronous reads, overflow will return an error
		ReadAsyncCap int

		// 最大读取的消息内容长度
		// Maximum read message content length
		ReadMaxPayloadSize int

		// 读缓冲区的大小
		// Size of the read buffer
		ReadBufferSize int

		// 异步写的容量限制, 容量溢出将会返回错误
		// Capacity limit for asynchronous writes, overflow will return an error
		WriteAsyncCap int

		// 最大写入的消息内容长度
		// Maximum length of written message content
		WriteMaxPayloadSize int

		// 写缓冲区的大小, v1.4.5版本此参数被废弃
		// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
		WriteBufferSize int

		// 是否开启数据压缩
		// Whether to turn on data compression
		CompressEnabled bool

		// 压缩阈值, 低于阈值的消息不会被压缩
		// Compression threshold, messages below the threshold will not be compressed
		CompressThreshold int

		// 是否检查文本utf8编码, 关闭性能会好点
		// Whether to check the text utf8 encoding, turn off the performance will be better
		CheckUtf8Enabled bool
	}

	ServerOption struct {
		// 写缓冲区的大小, v1.4.5版本此参数被废弃
		// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
		WriteBufferSize     int
		ReadAsyncEnabled    bool
		ReadAsyncGoLimit    int
		ReadAsyncCap        int
		ReadMaxPayloadSize  int
		ReadBufferSize      int
		WriteAsyncCap       int
		WriteMaxPayloadSize int
		CompressEnabled     bool
		CompressLevel       int
		CompressThreshold   int
		CheckUtf8Enabled    bool

		// WebSocket子协议, 一般不需要设置
		// WebSocket subprotocol, usually no need to set
		Subprotocols []string

		// 连接握手时添加的额外的响应头, 如果客户端不支持就不要传
		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		// attention: client may not support custom response header, use nil instead
		ResponseHeader http.Header

		// 鉴权
		// Authentication of requests for connection establishment
		Authorize func(r *http.Request, session SessionStorage) bool
	}
)

// Initialize 初始化配置
func (c *ServerOption) initialize() *ServerOption {
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.ReadAsyncGoLimit <= 0 {
		c.ReadAsyncGoLimit = defaultReadAsyncGoLimit
	}
	if c.ReadAsyncCap <= 0 {
		c.ReadAsyncCap = defaultReadAsyncCap
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}
	if c.WriteAsyncCap <= 0 {
		c.WriteAsyncCap = defaultWriteAsyncCap
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.CompressEnabled && c.CompressLevel == 0 {
		c.CompressLevel = defaultCompressLevel
	}
	if c.CompressThreshold <= 0 {
		c.CompressThreshold = defaultCompressThreshold
	}
	if c.Authorize == nil {
		c.Authorize = func(r *http.Request, session SessionStorage) bool {
			return true
		}
	}
	if c.ResponseHeader == nil {
		c.ResponseHeader = http.Header{}
	}
	return c
}

// 获取通用配置
func (c *ServerOption) getConfig() *Config {
	return &Config{
		ReadAsyncEnabled:    c.ReadAsyncEnabled,
		ReadAsyncGoLimit:    c.ReadAsyncGoLimit,
		ReadAsyncCap:        c.ReadAsyncCap,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		ReadBufferSize:      c.ReadBufferSize,
		WriteAsyncCap:       c.WriteAsyncCap,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		WriteBufferSize:     c.WriteBufferSize,
		CompressEnabled:     c.CompressEnabled,
		CompressThreshold:   c.CompressThreshold,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
	}
}

type ClientOption struct {
	// 写缓冲区的大小, v1.4.5版本此参数被废弃
	// Deprecated: Size of the write buffer, v1.4.5 version of this parameter is deprecated
	WriteBufferSize     int
	ReadAsyncEnabled    bool
	ReadAsyncGoLimit    int
	ReadAsyncCap        int
	ReadMaxPayloadSize  int
	ReadBufferSize      int
	WriteAsyncCap       int
	WriteMaxPayloadSize int
	CompressEnabled     bool
	CompressThreshold   int
	CheckUtf8Enabled    bool

	// 连接地址, 例如 wss://example.com/connect
	// service address, eg: wss://example.com/connect
	Addr string
	// 额外的请求头
	// extra request header
	RequestHeader http.Header
	// dial timeout
	// 连接超时时间
	DialTimeout time.Duration
	// TLS设置
	// tls config
	TlsConfig *tls.Config
}

func (c *ClientOption) initialize() *ClientOption {
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.ReadAsyncGoLimit <= 0 {
		c.ReadAsyncGoLimit = defaultReadAsyncGoLimit
	}
	if c.ReadAsyncCap <= 0 {
		c.ReadAsyncCap = defaultReadAsyncCap
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}
	if c.WriteAsyncCap <= 0 {
		c.WriteAsyncCap = defaultWriteAsyncCap
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.CompressThreshold <= 0 {
		c.CompressThreshold = defaultCompressThreshold
	}

	if c.DialTimeout <= 0 {
		c.DialTimeout = defaultDialTimeout
	}
	if c.RequestHeader == nil {
		c.RequestHeader = http.Header{}
	}
	return c
}

func (c *ClientOption) getConfig() *Config {
	return &Config{
		ReadAsyncEnabled:    c.ReadAsyncEnabled,
		ReadAsyncGoLimit:    c.ReadAsyncGoLimit,
		ReadAsyncCap:        c.ReadAsyncCap,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		ReadBufferSize:      c.ReadBufferSize,
		WriteAsyncCap:       c.WriteAsyncCap,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		WriteBufferSize:     c.WriteBufferSize,
		CompressEnabled:     c.CompressEnabled,
		CompressThreshold:   c.CompressThreshold,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
	}
}
