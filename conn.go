package gws

import (
	"bufio"
	"context"
	"github.com/lxzan/concurrency"
	"github.com/lxzan/gws/internal"
	"net"
	"sync"
	"time"
)

type Conn struct {
	// store session information
	Storage *internal.Map

	// context
	ctx context.Context
	// whether you use compression
	compressEnabled bool
	// tcp connection
	netConn net.Conn
	// server configs
	configs *Upgrader

	// read buffer
	rbuf *bufio.Reader
	// flate decompressors
	decompressor *decompressor
	// opcode for fragment frame
	continuationOpcode Opcode
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

	// message queue
	mq *concurrency.WorkerQueue

	// WebSocket EventHandler
	handler Event
}

func serveWebSocket(ctx context.Context, u *Upgrader, r *Request, netConn net.Conn, brw *bufio.ReadWriter, handler Event, compressEnabled bool) *Conn {
	c := &Conn{
		ctx:             ctx,
		Storage:         r.Storage,
		configs:         u,
		compressEnabled: compressEnabled,
		netConn:         netConn,
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

	var options = []concurrency.Option{concurrency.WithContext(ctx), concurrency.WithConcurrency(int64(u.Concurrency))}
	if u.Recovery {
		options = append(options, concurrency.WithRecovery())
	}
	c.mq = concurrency.NewWorkerQueue(options...)

	c.handler.OnOpen(c)

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

// Close
func (c *Conn) Close() error {
	return c.netConn.Close()
}

// set connection deadline
func (c *Conn) SetDeadline(t time.Time) {
	c.emitError(c.netConn.SetDeadline(t))
}

func (c *Conn) LocalAddr() net.Addr {
	return c.netConn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.netConn.RemoteAddr()
}
