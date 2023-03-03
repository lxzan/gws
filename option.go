package gws

import "net/http"

type (
	// Upgrader websocket upgrader
	Config struct {
		// 是否开启异步读, 开启的话会并行调用OnMessage
		ReadAsyncEnabled bool

		// 异步读的最大并行协程数量
		ReadAsyncGoLimit int

		// 异步读的容量限制, 容量溢出将会返回错误
		ReadAsyncCap int

		// 最大读取的消息内容长度
		ReadMaxPayloadSize int

		// 异步写的容量限制, 容量溢出将会返回错误
		WriteAsyncCap int

		// 最大写入的消息内容长度
		WriteMaxPayloadSize int

		// 是否开启数据压缩
		CompressEnabled bool

		// 压缩级别
		CompressLevel int

		// 压缩阈值, 低于阈值的消息不会被压缩
		CompressThreshold int

		// 是否检查文本utf8编码, 关闭性能会好点
		CheckUtf8Enabled bool
	}

	ServerOption struct {
		ReadAsyncEnabled    bool
		ReadAsyncGoLimit    int
		ReadAsyncCap        int
		ReadMaxPayloadSize  int
		WriteAsyncCap       int
		WriteMaxPayloadSize int
		CompressEnabled     bool
		CompressLevel       int
		CompressThreshold   int
		CheckUtf8Enabled    bool

		// 连接握手时添加的额外的响应头, 如果客户端不支持就不要传
		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		// attention: client may not support custom response header, use nil instead
		ResponseHeader http.Header

		// 检查请求来源
		// Check the origin of the request
		CheckOrigin func(r *Request) bool
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
	if c.WriteAsyncCap <= 0 {
		c.WriteAsyncCap = defaultWriteAsyncCap
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.CompressEnabled && c.CompressLevel == 0 {
		c.CompressLevel = defaultCompressLevel
	}
	if c.CompressThreshold <= 0 {
		c.CompressThreshold = defaultCompressionThreshold
	}
	if c.CheckOrigin == nil {
		c.CheckOrigin = func(r *Request) bool {
			return true
		}
	}
	if c.ResponseHeader == nil {
		c.ResponseHeader = http.Header{}
	}
	return c
}

func (c *ServerOption) ToConfig() *Config {
	return &Config{
		ReadAsyncEnabled:    c.ReadAsyncEnabled,
		ReadAsyncGoLimit:    c.ReadAsyncGoLimit,
		ReadAsyncCap:        c.ReadAsyncCap,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		WriteAsyncCap:       c.WriteAsyncCap,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		CompressEnabled:     c.CompressEnabled,
		CompressLevel:       c.CompressLevel,
		CompressThreshold:   c.CompressThreshold,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
	}
}
