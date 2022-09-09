package websocket

import (
	"time"
)

var _config = defaultConfig

type Config struct {
	Compress         bool
	HandshakeTimeout time.Duration
	WriteBufferSize  int
	ReadBufferSize   int
	MaxContentLength int
}

var defaultConfig = &Config{
	HandshakeTimeout: 3 * time.Second,
	Compress:         false,
	WriteBufferSize:  4 * 1024,
	ReadBufferSize:   4 * 1024,
	MaxContentLength: 10 * 1024 * 1024, // 10MB
}

func SetConfig(c *Config) {
	_config = c.init()
}

func (c *Config) init() *Config {
	var d = defaultConfig
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = d.HandshakeTimeout
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = d.WriteBufferSize
	}
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = d.ReadBufferSize
	}
	if c.MaxContentLength <= 0 {
		c.MaxContentLength = d.MaxContentLength
	}
	return c
}
