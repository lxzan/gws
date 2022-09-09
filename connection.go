package websocket

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"log"
	"net"
	"sync"
)

const compressorNum = 8

type Conn struct {
	// store session information
	Storage *sync.Map
	// websocket protocol upgrader
	conf *Upgrader
	// distinguish server/client side
	side uint8
	// make sure to exit only once
	onceClose sync.Once
	// whether you use compression
	compress bool
	// websocket event handler
	handler EventHandler
	// tcp connection
	netConn net.Conn

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

func serveWebSocket(u *Upgrader, r *Request, netConn net.Conn, compress bool, side uint8, handler EventHandler) {
	c := &Conn{
		fh:             frameHeader{},
		Storage:        r.Storage,
		conf:           u,
		side:           side,
		mu:             sync.Mutex{},
		onceClose:      sync.Once{},
		compress:       compress && _config.Compress,
		netConn:        netConn,
		handler:        handler,
		fragmentBuffer: bytes.NewBuffer(nil),
		mq:             internal.NewQueue(compressorNum),
	}

	// 节省资源
	if c.compress {
		c.compressors = newCompressors(compressorNum, _config.WriteBufferSize)
		c.decompressors = newDecompressors(compressorNum, _config.ReadBufferSize)
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

func (c *Conn) debugLog(err error) {
	if _config.LogEnabled && err != nil {
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

// 不要压缩控制帧
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
