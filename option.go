package gws

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

const (
	defaultParallelGolimit     = 8              // 默认并行协程限制
	defaultCompressLevel       = flate.BestSpeed // 默认压缩级别
	defaultReadMaxPayloadSize  = 16 * 1024 * 1024 // 默认读取最大负载大小
	defaultWriteMaxPayloadSize = 16 * 1024 * 1024 // 默认写入最大负载大小
	defaultCompressThreshold   = 512              // 默认压缩阈值
	defaultCompressorPoolSize  = 32               // 默认压缩器池大小
	defaultReadBufferSize      = 4 * 1024         // 默认读缓冲区大小
	defaultWriteBufferSize     = 4 * 1024         // 默认写缓冲区大小
	defaultHandshakeTimeout    = 5 * time.Second  // 默认握手超时
	defaultDialTimeout         = 5 * time.Second  // 默认拨号超时
)

type (
	// PermessageDeflate 压缩扩展配置
	// 客户端建议开启上下文接管, 不修改滑动窗口指数以提供最佳兼容性
	// 服务端开启上下文接管时每个连接占用更多内存, 需合理配置滑动窗口指数
	PermessageDeflate struct {
		Enabled               bool // 是否开启压缩
		Level                 int  // 压缩级别
		Threshold             int  // 压缩阈值, 小于阈值的消息不压缩, 仅适用于无上下文接管模式
		PoolSize              int  // 压缩器内存池大小, 越大竞争概率越低但内存消耗越大
		ServerContextTakeover bool // 服务端上下文接管
		ClientContextTakeover bool // 客户端上下文接管
		ServerMaxWindowBits   int  // 服务端滑动窗口指数, 取值范围 8<=n<=15, 表示 pow(2,n) 字节
		ClientMaxWindowBits   int  // 客户端滑动窗口指数, 取值范围 8<=n<=15, 表示 pow(2,n) 字节
	}

	brPoolIface interface {
		Get() *bufio.Reader
		Put(*bufio.Reader)
	}

	Config struct {
		brPool              brPoolIface                      // bufio.Reader 内存池
		bdPool              *internal.Pool[*bigDeflater]     // 大文件压缩器池
		cswPool             *internal.Pool[[]byte]           // 压缩器滑动窗口内存池
		dswPool             *internal.Pool[[]byte]           // 解压器滑动窗口内存池
		ParallelEnabled     bool                             // 是否开启并行消息处理
		ParallelGolimit     int                              // 单连接并行协程数量限制
		ReadMaxPayloadSize  int                              // 最大读取消息长度
		ReadBufferSize      int                              // 读缓冲区大小
		WriteMaxPayloadSize int                              // 最大写入消息长度
		WriteBufferSize     int                              // 写缓冲区大小 (v1.4.5 已废弃)
		CheckUtf8Enabled    bool                             // 是否检查文本 UTF-8 编码
		Recovery            func(logger Logger)              // OnMessage 恢复程序
		Logger              Logger                           // 日志工具
	}

	// ServerOption 服务端配置
	ServerOption struct {
		config              *Config              // 内部配置
		WriteBufferSize     int                  // 写缓冲区大小 (v1.4.5 已废弃)
		PermessageDeflate   PermessageDeflate    // 压缩扩展配置
		ParallelEnabled     bool                 // 是否启用并行处理
		ParallelGolimit     int                  // 并行协程限制
		ReadMaxPayloadSize  int                  // 读取最大负载大小
		ReadBufferSize      int                  // 读取缓冲区大小
		WriteMaxPayloadSize int                  // 写入最大负载大小
		CheckUtf8Enabled    bool                 // 是否启用 UTF-8 检查
		Logger              Logger               // 日志记录器
		Recovery            func(logger Logger)  // 恢复函数
		TlsConfig           *tls.Config          // TLS 配置
		HandshakeTimeout    time.Duration        // 握手超时时间
		SubProtocols        []string             // WebSocket 子协议, 握手失败会断开连接
		ResponseHeader      http.Header          // 额外响应头 (https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3)
		Authorize           func(r *http.Request, session SessionStorage) bool // 鉴权函数
		NewSession          func() SessionStorage // 创建 Session 存储空间
	}
)

// setThreshold 设置压缩阈值
// 开启上下文接管时必须压缩所有消息, 否则浏览器会报错
func (c *PermessageDeflate) setThreshold(isServer bool) {
	if (isServer && c.ServerContextTakeover) || (!isServer && c.ClientContextTakeover) {
		c.Threshold = 0
	}
}

// deleteProtectedHeaders 删除受保护的 WebSocket 头部字段
func (c *ServerOption) deleteProtectedHeaders() {
	c.ResponseHeader.Del(internal.Upgrade.Key)
	c.ResponseHeader.Del(internal.Connection.Key)
	c.ResponseHeader.Del(internal.SecWebSocketAccept.Key)
	c.ResponseHeader.Del(internal.SecWebSocketExtensions.Key)
	c.ResponseHeader.Del(internal.SecWebSocketProtocol.Key)
}

// initServerOption 初始化服务端配置
func initServerOption(c *ServerOption) *ServerOption {
	if c == nil {
		c = new(ServerOption)
	}
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.ParallelGolimit <= 0 {
		c.ParallelGolimit = defaultParallelGolimit
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.Authorize == nil {
		c.Authorize = func(r *http.Request, session SessionStorage) bool { return true }
	}
	if c.NewSession == nil {
		c.NewSession = func() SessionStorage { return newSmap() }
	}
	if c.ResponseHeader == nil {
		c.ResponseHeader = http.Header{}
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}
	if c.Logger == nil {
		c.Logger = defaultLogger
	}
	if c.Recovery == nil {
		c.Recovery = Recovery
	}

	if c.PermessageDeflate.Enabled {
		if c.PermessageDeflate.ServerMaxWindowBits < 8 || c.PermessageDeflate.ServerMaxWindowBits > 15 {
			c.PermessageDeflate.ServerMaxWindowBits = internal.SelectValue(c.PermessageDeflate.ServerContextTakeover, 12, 15)
		}
		if c.PermessageDeflate.ClientMaxWindowBits < 8 || c.PermessageDeflate.ClientMaxWindowBits > 15 {
			c.PermessageDeflate.ClientMaxWindowBits = internal.SelectValue(c.PermessageDeflate.ClientContextTakeover, 12, 15)
		}
		if c.PermessageDeflate.Threshold <= 0 {
			c.PermessageDeflate.Threshold = defaultCompressThreshold
		}
		if c.PermessageDeflate.Level == 0 {
			c.PermessageDeflate.Level = defaultCompressLevel
		}
		if c.PermessageDeflate.Level < flate.HuffmanOnly || c.PermessageDeflate.Level > flate.BestCompression {
			panic(fmt.Sprintf("gws: invalid compress level: %d", c.PermessageDeflate.Level))
		}
		if c.PermessageDeflate.PoolSize <= 0 {
			c.PermessageDeflate.PoolSize = defaultCompressorPoolSize
		}
		c.PermessageDeflate.PoolSize = internal.ToBinaryNumber(c.PermessageDeflate.PoolSize)
	}

	c.deleteProtectedHeaders()

	c.config = &Config{
		ParallelEnabled:     c.ParallelEnabled,
		ParallelGolimit:     c.ParallelGolimit,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		ReadBufferSize:      c.ReadBufferSize,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		WriteBufferSize:     c.WriteBufferSize,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
		Recovery:            c.Recovery,
		Logger:              c.Logger,
		brPool: internal.NewPool(func() *bufio.Reader {
			return bufio.NewReaderSize(nil, c.ReadBufferSize)
		}),
	}

	if c.PermessageDeflate.Enabled {
		c.config.bdPool = internal.NewPool(func() *bigDeflater {
			return newBigDeflater(true, c.PermessageDeflate)
		})
		if c.PermessageDeflate.ServerContextTakeover {
			windowSize := internal.BinaryPow(c.PermessageDeflate.ServerMaxWindowBits)
			c.config.cswPool = internal.NewPool(func() []byte {
				return make([]byte, 0, windowSize)
			})
		}
		if c.PermessageDeflate.ClientContextTakeover {
			windowSize := internal.BinaryPow(c.PermessageDeflate.ClientMaxWindowBits)
			c.config.dswPool = internal.NewPool(func() []byte {
				return make([]byte, 0, windowSize)
			})
		}
	}

	return c
}

// getConfig 获取服务端配置
func (c *ServerOption) getConfig() *Config { return c.config }

// ClientOption 客户端配置
type ClientOption struct {
	WriteBufferSize     int                  // 写缓冲区大小 (v1.4.5 已废弃)
	PermessageDeflate   PermessageDeflate    // 压缩扩展配置
	ParallelEnabled     bool                 // 是否启用并行处理
	ParallelGolimit     int                  // 并行协程限制
	ReadMaxPayloadSize  int                  // 读取最大负载大小
	ReadBufferSize      int                  // 读取缓冲区大小
	WriteMaxPayloadSize int                  // 写入最大负载大小
	CheckUtf8Enabled    bool                 // 是否启用 UTF-8 检查
	Logger              Logger               // 日志记录器
	Recovery            func(logger Logger)  // 恢复函数
	Addr                string               // 连接地址, 例如 wss://example.com/connect
	RequestHeader       http.Header          // 额外请求头
	HandshakeTimeout    time.Duration        // 握手超时时间
	TlsConfig           *tls.Config          // TLS 配置
	NewDialer           func() (Dialer, error) // 拨号器, 默认返回 net.Dialer, 也可用于设置代理
	NewSession          func() SessionStorage // 创建 Session 存储空间
}

// initClientOption 初始化客户端配置
func initClientOption(c *ClientOption) *ClientOption {
	if c == nil {
		c = new(ClientOption)
	}
	if c.ReadMaxPayloadSize <= 0 {
		c.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.ParallelGolimit <= 0 {
		c.ParallelGolimit = defaultParallelGolimit
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = defaultReadBufferSize
	}
	if c.WriteMaxPayloadSize <= 0 {
		c.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = defaultWriteBufferSize
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}
	if c.RequestHeader == nil {
		c.RequestHeader = http.Header{}
	}
	if c.NewDialer == nil {
		c.NewDialer = func() (Dialer, error) { return &net.Dialer{Timeout: defaultDialTimeout}, nil }
	}
	if c.NewSession == nil {
		c.NewSession = func() SessionStorage { return newSmap() }
	}
	if c.Logger == nil {
		c.Logger = defaultLogger
	}
	if c.Recovery == nil {
		c.Recovery = Recovery
	}
	if c.PermessageDeflate.Enabled {
		if c.PermessageDeflate.ServerMaxWindowBits < 8 || c.PermessageDeflate.ServerMaxWindowBits > 15 {
			c.PermessageDeflate.ServerMaxWindowBits = 15
		}
		if c.PermessageDeflate.ClientMaxWindowBits < 8 || c.PermessageDeflate.ClientMaxWindowBits > 15 {
			c.PermessageDeflate.ClientMaxWindowBits = 15
		}
		if c.PermessageDeflate.Threshold <= 0 {
			c.PermessageDeflate.Threshold = defaultCompressThreshold
		}
		if c.PermessageDeflate.Level == 0 {
			c.PermessageDeflate.Level = defaultCompressLevel
		}
		if c.PermessageDeflate.Level < flate.HuffmanOnly || c.PermessageDeflate.Level > flate.BestCompression {
			panic(fmt.Sprintf("gws: invalid compress level: %d", c.PermessageDeflate.Level))
		}
		c.PermessageDeflate.PoolSize = 1
	}
	return c
}

// getConfig 将 ClientOption 配置转换为 Config
func (c *ClientOption) getConfig() *Config {
	config := &Config{
		ParallelEnabled:     c.ParallelEnabled,
		ParallelGolimit:     c.ParallelGolimit,
		ReadMaxPayloadSize:  c.ReadMaxPayloadSize,
		ReadBufferSize:      c.ReadBufferSize,
		WriteMaxPayloadSize: c.WriteMaxPayloadSize,
		WriteBufferSize:     c.WriteBufferSize,
		CheckUtf8Enabled:    c.CheckUtf8Enabled,
		Recovery:            c.Recovery,
		Logger:              c.Logger,
	}
	return config
}
