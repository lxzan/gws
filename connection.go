package gws

import (
	"bufio"
	"context"
	"github.com/lxzan/gws/internal"
	"net"
	"sync"
	"time"
)

type Conn struct {
	// context
	ctx context.Context
	// store session information
	Storage *internal.Map
	// message channel
	messageChan chan *Message
	// whether you use compression
	compressEnabled bool
	// tcp connection
	netConn net.Conn
	// server configs
	configs *Upgrader
	// last ping time
	pingTime time.Time
	// ping count
	pingCount int

	// read buffer
	rbuf *bufio.Reader
	// flate decompressors
	decompressor *decompressor
	// opcode for fragment frame
	continuationOpcode Opcode
	// continuation frame
	continuationBuffer *internal.Buffer
	// frame payload for read control frame
	controlBuffer [internal.Bv7]byte
	// frame header for read
	fh frameHeader

	// write lock
	wmu sync.Mutex
	// flate compressors
	compressor *compressor
	// write buffer
	wbuf *bufio.Writer
}

func serveWebSocket(ctx context.Context, u *Upgrader, r *Request, netConn net.Conn, brw *bufio.ReadWriter, compressEnabled bool) *Conn {
	c := &Conn{
		ctx:                ctx,
		Storage:            r.Storage,
		configs:            u,
		messageChan:        make(chan *Message, u.MessageChannelBufferSize),
		compressEnabled:    compressEnabled,
		netConn:            netConn,
		wbuf:               brw.Writer,
		wmu:                sync.Mutex{},
		rbuf:               brw.Reader,
		fh:                 frameHeader{},
		continuationBuffer: internal.NewBuffer(nil),
	}
	if c.compressEnabled {
		c.compressor = newCompressor()
		c.decompressor = newDecompressor()
	}

	go func() {
		for {
			if err := c.readMessage(); err != nil {
				c.emitError(err)
				return
			}
			if err := c.netConn.SetReadDeadline(time.Time{}); err != nil {
				c.emitError(err)
				return
			}
		}
	}()

	return c
}

func (c *Conn) isCanceled() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

func (c *Conn) Close() error {
	return c.netConn.Close()
}

// set connection deadline
func (c *Conn) SetDeadline(d time.Duration) error {
	return c.netConn.SetDeadline(time.Now().Add(d))
}

func (c *Conn) LocalAddr() net.Addr {
	return c.netConn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.netConn.RemoteAddr()
}
