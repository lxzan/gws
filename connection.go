package websocket

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"log"
	"net"
	"sync"
)

type Conn struct {
	// store session information
	Storage *internal.Map
	// websocket protocol upgrader
	conf *ServerOptions
	// distinguish server/client side
	side uint8
	// make sure to exit only once
	onceClose sync.Once
	// whether you use compression
	compressEnabled bool
	// websocket event handler
	handler EventHandler
	// tcp connection
	netConn net.Conn
	// websocket middlewares
	middlewares []HandlerFunc

	// opcode for fragment
	opcode Opcode
	// message queue
	mq *internal.Queue
	// flate decompressors
	decompressors *decompressors
	// continuation frame for read
	fragmentBuffer *bytes.Buffer
	// frame payload for read control frame
	controlBuffer [internal.PayloadSizeLv1 + 4]byte
	// frame header for read
	fh frameHeader

	// write lock
	mu sync.Mutex
	// flate compressors
	compressors *compressors
}

func serveWebSocket(conf *Upgrader, r *Request, netConn net.Conn, compressEnabled bool, handler EventHandler) {
	c := &Conn{
		fh:              frameHeader{},
		Storage:         r.Storage,
		conf:            conf.ServerOptions,
		side:            serverSide,
		mu:              sync.Mutex{},
		onceClose:       sync.Once{},
		compressEnabled: compressEnabled,
		netConn:         netConn,
		handler:         handler,
		fragmentBuffer:  bytes.NewBuffer(nil),
		mq:              internal.NewQueue(int64(conf.Concurrency)),
		middlewares:     conf.middlewares,
	}

	// 为节省资源, 动态初始化压缩器
	// To save resources, dynamically initialize the compressor
	if c.compressEnabled {
		c.compressors = newCompressors(int(conf.Concurrency), conf.WriteBufferSize)
		c.decompressors = newDecompressors(int(conf.Concurrency), conf.ReadBufferSize)
	}

	handler.OnOpen(c)

	for {
		continued, err := c.readMessage()
		if err != nil {
			c.emitError(err)
			return
		}
		if !continued {
			return
		}
	}
}

// print debug log
func (c *Conn) debugLog(err error) {
	if c.conf.LogEnabled && err != nil {
		log.Printf("websocket error: " + err.Error())
	}
}

func (c *Conn) emitError(err error) {
	if err == nil {
		return
	}

	code, ok := err.(Code)
	if !ok {
		code = CloseGoingAway
	}

	// try to send close message
	c.Close(code, nil)
	c.handler.OnError(c, err)
}

func (c *Conn) Raw() net.Conn {
	return c.netConn
}

// close the connection
func (c *Conn) Close(code Code, reason []byte) (err error) {
	c.onceClose.Do(func() {
		var content = code.Bytes()
		if len(content) > 0 {
			content = append(content, reason...)
		} else {
			content = append(content, code.Error()...)
		}
		_ = c.writeFrame(Opcode_CloseConnection, content, false)
		err = c.netConn.Close()
	})
	return
}
