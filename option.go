package gws

import (
	"net/http"
)

type Option func(c *Upgrader)

// WithEventHandler 设置事件处理器
// set event handler
func WithEventHandler(eventHandler Event) Option {
	return func(c *Upgrader) {
		c.EventHandler = eventHandler
	}
}

// WithAsyncIOGoLimit 设置单个连接异步IO的最大并发协程数量限制
// set the maximum number of concurrent co-processes for asynchronous read
func WithAsyncIOGoLimit(limit int) Option {
	return func(c *Upgrader) {
		c.AsyncIOGoLimit = limit
	}
}

// WithAsyncReadEnabled 开启异步读功能, 并发地调用onmessage, 并发度会受到AIOGoroutineLimit的限制.
// enable asynchronous read, call onmessage concurrently, concurrency is limited by AIOGoroutineLimit.
func WithAsyncReadEnabled() Option {
	return func(c *Upgrader) {
		c.AsyncReadEnabled = true
	}
}

// WithCompress 设置数据压缩. 是否压缩, 压缩级别和阈值, 低于阈值的数据不会被压缩.
// set data compression.
// set the compression level and the threshold value, below which the data will not be compressed.
func WithCompress(level int, threshold int) Option {
	return func(c *Upgrader) {
		c.CompressEnabled = true
		c.CompressLevel = level
		c.CompressionThreshold = threshold
	}
}

// WithMaxContentLength 设置消息最大长度(字节)
// set max content length (byte).
func WithMaxContentLength(n int) Option {
	return func(c *Upgrader) {
		c.MaxContentLength = n
	}
}

// WithCheckTextEncoding 检查文本utf8编码, 关闭性能会更好.
// set text encoding checking
func WithCheckTextEncoding() Option {
	return func(c *Upgrader) {
		c.CheckTextEncoding = true
	}
}

// WithResponseHeader 设置响应头, 客户端可能不支持.
// set response header, client may not support, use nil instead
func WithResponseHeader(h http.Header) Option {
	return func(c *Upgrader) {
		c.ResponseHeader = h
	}
}

// WithCheckOrigin 检查请求来源, 进行鉴权.
// check request origin
func WithCheckOrigin(f func(r *Request) bool) Option {
	return func(c *Upgrader) {
		c.CheckOrigin = f
	}
}
