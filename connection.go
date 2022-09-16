package gws

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/lxzan/gws/internal"
	"log"
	"net"
	"sync"
	"time"
)

type Conn struct {
	// context
	Context context.Context
	// store session information
	Storage *internal.Map

	// cancel func
	cancel func()
	// websocket protocol upgrader
	conf *ServerOptions
	// make sure to exit only once
	onceClose sync.Once
	// make sure print log only once
	onceLog sync.Once
	// whether you use compression
	compressEnabled bool
	// websocket event handler
	handler EventHandler
	// tcp connection
	netConn net.Conn
	// websocket middlewares
	middlewares []HandlerFunc

	// read buffer
	rbuf *bufio.Reader
	// message queue
	mq *internal.Queue
	// flate decompressors
	decompressors *decompressors
	// opcode for fragment frame
	opcode Opcode
	// continuation frame
	fragment *internal.Buffer
	// frame payload for read control frame
	controlBuffer [internal.Bv7]byte
	// frame header for read
	fh frameHeader

	// write lock
	wmu sync.Mutex
	// flate compressors
	compressors *compressors
	// write buffer
	wbuf *bufio.Writer
	// flush write buffer
	wtimer *time.Timer
}

func serveWebSocket(conf *Upgrader, r *Request, netConn net.Conn, brw *bufio.ReadWriter, compressEnabled bool, handler EventHandler) *Conn {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Conn{
		Context:         ctx,
		cancel:          cancel,
		Storage:         r.Storage,
		conf:            conf.ServerOptions,
		onceClose:       sync.Once{},
		onceLog:         sync.Once{},
		compressEnabled: compressEnabled,
		netConn:         netConn,
		handler:         handler,
		middlewares:     conf.middlewares,
		wbuf:            brw.Writer,
		wmu:             sync.Mutex{},
		rbuf:            brw.Reader,
		fh:              frameHeader{},
		mq:              internal.NewQueue(int64(conf.Concurrency)),
		fragment:        internal.NewBuffer(nil),
	}

	c.wtimer = time.AfterFunc(conf.FlushLatency, func() { c.flush() })

	// 为节省资源, 动态初始化压缩器
	// To save resources, dynamically initialize the compressor
	if c.compressEnabled {
		c.compressors = newCompressors(int(conf.Concurrency))
		c.decompressors = newDecompressors(int(conf.Concurrency))
	}

	handler.OnOpen(c)

	go func(socket *Conn) {
		defer func() {
			_ = c.netConn.Close()
			socket.wtimer.Stop()
			socket.cancel()
		}()

		for {
			continued, err := socket.readMessage()
			if err != nil {
				socket.emitError(err)
				return
			}
			if !continued {
				return
			}
		}
	}(c)

	return c
}

func (c *Conn) isCanceled() bool {
	select {
	case <-c.Context.Done():
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

func (c *Conn) emitError(err error) {
	if err == nil {
		return
	}

	code, ok := err.(Code)
	if !ok {
		c.debugLog(err)
		code = CloseGoingAway
	}

	// try to send close message
	c.onceClose.Do(func() {
		c.writeClose(code, nil)
		c.handler.OnError(c, err)
	})
}

func (c *Conn) Close(code Code, reason []byte) error {
	var str = ""
	if len(reason) == 0 {
		str = code.Error()
	}

	c.onceClose.Do(func() {
		var msg = fmt.Sprintf("received close frame, code=%d, reason=%s", code.Uint16(), str)
		c.debugLog(errors.New(msg))
		c.writeClose(code, reason)
		c.handler.OnClose(c, code, reason)
	})
	return nil
}

func (c *Conn) writeClose(code Code, reason []byte) {
	var content = code.Bytes()
	if len(content) > 0 {
		content = append(content, reason...)
	} else {
		content = append(content, code.Error()...)
	}
	_ = c.writeFrame(OpcodeCloseConnection, content, false)
	c.flush()
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
