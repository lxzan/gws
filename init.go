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

	// max message length, dv=1024*1024 (1MB)
	MaxContentLength int

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
	MaxContentLength: 1 * 1024 * 1024, // 1MB
}

func (c *ServerOptions) init() {
	var d = defaultConfig
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = d.HandshakeTimeout
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
}
