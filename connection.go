package websocket

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"io"
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

	// message queue
	mq *internal.Queue
	// flate decompressors
	decompressors *decompressors
	// use for continuation frame
	fragmentBuffer *bytes.Buffer
	// read control frame
	controlBuffer [internal.PayloadSizeLv1]byte
	// header
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
		compress:       compress,
		netConn:        netConn,
		handler:        handler,
		fragmentBuffer: bytes.NewBuffer(nil),
		mq:             internal.NewQueue(compressorNum),
		compressors:    newCompressors(compressorNum, _config.WriteBufferSize),
		decompressors:  newDecompressors(compressorNum, _config.ReadBufferSize),
	}

	handler.OnConnect(c)

	for {
		if err := c.readMessage(); err != nil {
			c.emitError(err)
			return
		}
	}
}

func (c *Conn) emitError(err error) {
	// has been handled
	if err == ERR_DISCONNECT {
		return
	}

	// try to send close message
	if code, ok := err.(Code); ok {
		_ = c.Close(code, nil)
	} else {
		switch err {
		case io.EOF:
			_ = c.Close(CloseGoingAway, nil)
		default:
			_ = c.Close(CloseAbnormalClosure, nil)
		}
	}

	c.handler.OnError(c, err)
	return
}

func (c *Conn) Raw() net.Conn {
	return c.netConn
}

// 不要压缩控制帧
func (c *Conn) Close(code Code, reason []byte) (err error) {
	c.onceClose.Do(func() {
		var content = code.Bytes()
		content = append(content, reason...)
		_ = c.Write(Opcode_CloseConnection, content)
		err = c.netConn.Close()
	})
	return
}
