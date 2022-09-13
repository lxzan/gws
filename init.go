package gws

import (
	"compress/flate"
	"github.com/lxzan/gws/internal"
	"time"
)

var _pool = internal.NewBufferPool()

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

	// number of concurrently processed messages allowed by the connection, dv=4
	// Concurrency=pow(2, n), eg: 4, 8, 16...
	Concurrency uint8

	// read frame timeout, dv=5s
	ReadTimeout time.Duration

	// write frame timeout, dv=5s
	WriteTimeout time.Duration
}

var defaultConfig = ServerOptions{
	LogEnabled:       true,
	HandshakeTimeout: 3 * time.Second,
	ReadTimeout:      5 * time.Second,
	WriteTimeout:     5 * time.Second,
	CompressEnabled:  false,
	CompressLevel:    flate.BestSpeed,
	Concurrency:      4,
	WriteBufferSize:  4 * 1024,        // 4KB
	ReadBufferSize:   4 * 1024,        // 4KB
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
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = d.ReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = d.WriteTimeout
	}
	if c.Concurrency == 0 {
		c.Concurrency = d.Concurrency
	}
}
