package gws

import "net/http"

type Option func(c *Upgrader)

// WithInitialize initialize the upgrader configure
func WithInitialize() Option {
	return func(c *Upgrader) {
		if c.ResponseHeader == nil {
			c.ResponseHeader = http.Header{}
		}
		if c.CheckOrigin == nil {
			c.CheckOrigin = func(r *Request) bool {
				return true
			}
		}
		if c.MaxContentLength <= 0 {
			c.MaxContentLength = defaultMaxContentLength
		}
		if c.CompressEnabled && c.CompressLevel == 0 {
			c.CompressLevel = defaultCompressLevel
		}
	}
}

// WithEventHandler set event handler
func WithEventHandler(eventHandler Event) Option {
	return func(c *Upgrader) {
		c.EventHandler = eventHandler
	}
}

// WithCompress set deflate compress
func WithCompress(enabled bool, level int) Option {
	return func(c *Upgrader) {
		c.CompressEnabled = enabled
		c.CompressLevel = level
	}
}

// WithMaxContentLength set max content length
func WithMaxContentLength(n int) Option {
	return func(c *Upgrader) {
		c.MaxContentLength = n
	}
}

// WithCheckTextEncoding set text encoding checking
func WithCheckTextEncoding(check bool) Option {
	return func(c *Upgrader) {
		c.CheckTextEncoding = check
	}
}

// WithResponseHeader set response header
// client may not support, use nil instead
func WithResponseHeader(h http.Header) Option {
	return func(c *Upgrader) {
		c.ResponseHeader = h
	}
}

// WithCheckOrigin check request origin
func WithCheckOrigin(f func(r *Request) bool) Option {
	return func(c *Upgrader) {
		c.CheckOrigin = f
	}
}
