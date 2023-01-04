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
	// store session information
	*Storage
	// context
	ctx context.Context
	// whether you use compression
	compressEnabled bool
	// tcp connection
	conn net.Conn
	// server configs
	configs *Upgrader
	// whether server is closed
	closed uint32

	// read buffer
	rbuf *bufio.Reader
	// flate decompressors
	decompressor *decompressor
	// opcode for fragment frame
	continuationOpcode Opcode
	// continuation is compressed
	continuationCompressed bool
	// continuation frame
	continuationBuffer *internal.Buffer
	// frame header for read
	fh frameHeader

	// write lock
	wmu sync.Mutex
	// flate compressors
	compressor *compressor
	// write buffer
	wbuf *bufio.Writer

	// WebSocket Event Handler
	handler Event
}

func serveWebSocket(ctx context.Context, u *Upgrader, r *Request, netConn net.Conn, brw *bufio.ReadWriter, handler Event, compressEnabled bool) {
	c := &Conn{
		ctx:             ctx,
		Storage:         r.Storage,
		configs:         u,
		compressEnabled: compressEnabled,
		conn:            netConn,
		closed:          0,
		wbuf:            brw.Writer,
		wmu:             sync.Mutex{},
		rbuf:            brw.Reader,
		fh:              frameHeader{},
		handler:         handler,
	}
	if c.compressEnabled {
		c.compressor = newCompressor(u.CompressLevel)
		c.decompressor = newDecompressor()
	}

	c.handler.OnOpen(c)

	for {
		if err := c.readMessage(); err != nil {
			c.emitError(err)
			return
		}
		if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
			c.emitError(err)
			return
		}
	}
}

func (c *Conn) isCanceled() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

// Close
func (c *Conn) Close() error {
	return c.conn.Close()
}

// SetDeadline sets deadline
func (c *Conn) SetDeadline(t time.Time) {
	c.emitError(c.conn.SetDeadline(t))
}

// SetReadDeadline sets read deadline
func (c *Conn) SetReadDeadline(t time.Time) {
	c.emitError(c.conn.SetReadDeadline(t))
}

// SetWriteDeadline sets write deadline
func (c *Conn) SetWriteDeadline(t time.Time) {
	c.emitError(c.conn.SetWriteDeadline(t))
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
