package websocket

import (
	"compress/flate"
	"time"
)

// dv means default value
type ServerOptions struct {
	// whether to show error log, dv=true
	LogEnabled bool

	// whether to compress data, dv = false
	CompressEnabled bool

	// compress level eg: flate.BestSpeed
	CompressLevel int

	// websocket  handshake timeout, dv=3s
	HandshakeTimeout time.Duration

	// read buffer size, dv=4KB
	ReadBufferSize int

	// write buffer size, dv=dv=4*1024 (4KB)
	WriteBufferSize int

	// max message length, dv=1024*1024 (1MiB)
	MaxContentLength int
}

var defaultConfig = ServerOptions{
	LogEnabled:       true,
	HandshakeTimeout: 3 * time.Second,
	CompressEnabled:  false,
	CompressLevel:    flate.BestSpeed,
	WriteBufferSize:  4 * 1024,
	ReadBufferSize:   4 * 1024,
	MaxContentLength: 1 * 1024 * 1024, // 1MB
}

func (c *ServerOptions) init() {
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
}
