package gws

import (
	"bufio"
	"context"
	"github.com/lxzan/gws/internal"
	"log"
	"net"
	"sync"
	"time"
)

type Conn struct {
	// context
	ctx context.Context
	// store session information
	storage *sync.Map
	// message channel
	messageChan chan *Message
	// websocket protocol upgrader
	conf *ServerOptions
	// make sure print log at most once
	onceLog sync.Once
	// whether you use compression
	compressEnabled bool
	// tcp connection
	netConn net.Conn

	// read buffer
	rbuf *bufio.Reader
	// message queue
	mq *internal.Queue
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

func serveWebSocket(ctx context.Context, conf *Upgrader, r *Request, netConn net.Conn, brw *bufio.ReadWriter, compressEnabled bool) *Conn {
	c := &Conn{
		ctx:                ctx,
		storage:            &sync.Map{},
		conf:               conf.ServerOptions,
		onceLog:            sync.Once{},
		compressEnabled:    compressEnabled,
		netConn:            netConn,
		wbuf:               brw.Writer,
		wmu:                sync.Mutex{},
		rbuf:               brw.Reader,
		fh:                 frameHeader{},
		mq:                 internal.NewQueue(int64(conf.Concurrency)),
		continuationBuffer: internal.NewBuffer(nil),
	}

	// 为节省资源, 动态初始化压缩器
	// To save resources, dynamically initialize the compressor
	if c.compressEnabled {
		c.compressor = newCompressor()
		c.decompressor = newDecompressor()
	}

	go func(socket *Conn) {
		for {
			if err := socket.readMessage(); err != nil {
				c.messageChan <- &Message{err: err}
				return
			}
		}
	}(c)

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

// print debug log
func (c *Conn) debugLog(err error) {
	if c.conf.LogEnabled && err != nil {
		c.onceLog.Do(func() {
			log.Printf("websocket: " + err.Error())
		})
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
