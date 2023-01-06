package gws

import (
	"bufio"
	"context"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Conn struct {
	// context
	ctx context.Context
	// whether you use compression
	compressEnabled bool
	// tcp connection
	conn net.Conn
	// server configs
	config Config
	// read buffer
	rbuf *bufio.Reader
	// flate decompressor
	decompressor *decompressor
	// opcode for fragment frame
	continuationOpcode Opcode
	// continuation is compressed
	continuationCompressed bool
	// continuation frame
	continuationBuffer *internal.Buffer
	// frame header for read
	fh frameHeader
	// write buffer
	wbuf *bufio.Writer
	// flate compressor
	compressor *compressor
	// WebSocket Event Handler
	handler Event

	// Concurrent Variable
	// store session information
	*internal.SessionStorage
	// whether server is closed
	closed uint32
	// write lock
	wmu *sync.Mutex
}

func serveWebSocket(ctx context.Context, config Config, r *internal.Request, netConn net.Conn, brw *bufio.ReadWriter, handler Event, compressEnabled bool) *Conn {
	c := &Conn{
		ctx:             ctx,
		SessionStorage:  r.SessionStorage,
		config:          config,
		compressEnabled: compressEnabled,
		conn:            netConn,
		closed:          0,
		wbuf:            brw.Writer,
		wmu:             &sync.Mutex{},
		rbuf:            brw.Reader,
		fh:              frameHeader{},
		handler:         handler,
	}
	if c.compressEnabled {
		c.compressor = newCompressor(config.CompressLevel)
		c.decompressor = newDecompressor()
	}
	c.handler.OnOpen(c)
	return c
}

// Listen listening to websocket messages through a dead loop
// 通过死循环监听websocket消息
func (c *Conn) Listen() {
	defer c.conn.Close()
	for {
		if atomic.LoadUint32(&c.closed) == 1 {
			return
		}
		if err := c.readMessage(); err != nil {
			c.emitError(err)
			return
		}
	}
}

func (c *Conn) emitError(err error) {
	if err == nil {
		return
	}
	code := internal.CloseNormalClosure
	v, ok := err.(internal.StatusCode)
	if ok {
		code = v
	}
	var content = code.Bytes()
	content = append(content, err.Error()...)
	if len(content) > internal.Lv1 {
		content = content[:internal.Lv1]
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		_ = c.writeMessage(OpcodeCloseConnection, content)
		_ = c.conn.SetDeadline(time.Now())
		c.handler.OnError(c, err)
	}
}

func (c *Conn) emitClose(msg *Message) error {
	var responseCode = internal.CloseNormalClosure
	var realCode = internal.CloseNormalClosure.Uint16()
	switch msg.buf.Len() {
	case 0:
		responseCode = 0
		realCode = 0
	case 1:
		responseCode = internal.CloseProtocolError
		realCode = uint16(msg.buf.Bytes()[0])
	default:
		var b [2]byte
		_, _ = msg.buf.Read(b[0:])
		realCode = binary.BigEndian.Uint16(b[0:])
		switch realCode {
		case 1004, 1005, 1006, 1014, 1015:
			responseCode = internal.CloseProtocolError
		default:
			if realCode < 1000 || realCode >= 5000 || (realCode >= 1016 && realCode < 3000) {
				responseCode = internal.CloseProtocolError
			} else if realCode < 1016 {
				responseCode = internal.CloseNormalClosure
			} else {
				responseCode = internal.StatusCode(realCode)
			}
		}
		if c.config.CheckTextEncoding && !msg.valid() {
			responseCode = internal.CloseUnsupportedData
		}
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		_ = c.writeMessage(OpcodeCloseConnection, responseCode.Bytes())
		c.handler.OnClose(c, realCode, msg.Bytes())
	}
	return internal.CloseNormalClosure
}

func (c *Conn) isCanceled() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
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
