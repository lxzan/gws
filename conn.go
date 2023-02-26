package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Conn struct {
	// whether to use compression
	compressEnabled bool
	// tcp connection
	conn net.Conn
	// server configs
	config *Upgrader
	// read buffer
	rbuf *bufio.Reader
	// flate decompressor
	decompressor *decompressor
	// opcode for fragment frame
	continuationOpcode Opcode
	// continuation is compressed
	continuationCompressed bool
	// continuation frame
	continuationBuffer *bytes.Buffer
	// frame header for read
	fh frameHeader
	// write buffer
	wbuf *bufio.Writer
	// flate compressor
	compressor *compressor
	// WebSocket Event Handler
	handler Event

	// store session information
	SessionStorage SessionStorage
	// whether server is closed
	closed uint32
	// write lock
	wmu *sync.Mutex
	// async read task queue
	readTaskQ *workerQueue
	// async write task queue
	writeTaskQ *workerQueue
	// write channel
	wChannel chan messageWrapper
}

func serveWebSocket(config *Upgrader, r *Request, netConn net.Conn, brw *bufio.ReadWriter, handler Event, compressEnabled bool) *Conn {
	c := &Conn{
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
		readTaskQ:       newWorkerQueue(int64(config.AsyncIOGoLimit)),
		writeTaskQ:      newWorkerQueue(1),
		wChannel:        make(chan messageWrapper, config.AsyncIOGoLimit),
	}
	if c.compressEnabled {
		c.compressor = newCompressor(config.CompressLevel)
		c.decompressor = newDecompressor()
	}

	// initialize the connection
	c.SetDeadline(time.Time{})
	c.SetReadDeadline(time.Time{})
	c.SetWriteDeadline(time.Time{})
	c.setNoDelay(c.conn)
	return c
}

// Listen listening to websocket messages through a dead loop
// 监听websocket消息
func (c *Conn) Listen() {
	defer c.conn.Close()

	c.handler.OnOpen(c)
	for {
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

	var responseCode = internal.CloseNormalClosure
	var responseErr error = internal.CloseNormalClosure
	switch v := err.(type) {
	case internal.StatusCode:
		responseCode = v
	case *internal.Error:
		responseCode = v.Code
		responseErr = v.Err
	default:
		responseErr = err
	}

	var content = responseCode.Bytes()
	content = append(content, err.Error()...)
	if len(content) > internal.ThresholdV1 {
		content = content[:internal.ThresholdV1]
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		_ = c.doWrite(OpcodeCloseConnection, content)
		c.handler.OnError(c, responseErr)
	}
}

func (c *Conn) emitClose(buf *bytes.Buffer) error {
	var responseCode = internal.CloseNormalClosure
	var realCode = internal.CloseNormalClosure.Uint16()
	switch buf.Len() {
	case 0:
		responseCode = 0
		realCode = 0
	case 1:
		responseCode = internal.CloseProtocolError
		realCode = uint16(buf.Bytes()[0])
		buf.Reset()
	default:
		var b [2]byte
		_, _ = buf.Read(b[0:])
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
		if c.config.CheckTextEncoding && !isTextValid(OpcodeCloseConnection, buf.Bytes()) {
			responseCode = internal.CloseUnsupportedData
		}
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		_ = c.doWrite(OpcodeCloseConnection, responseCode.Bytes())
		c.handler.OnClose(c, realCode, buf.Bytes())
	}
	return internal.CloseNormalClosure
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

// NetConn get tcp/tls/... conn
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// setNoDelay set tcp no delay
func (c *Conn) setNoDelay(conn net.Conn) {
	switch v := conn.(type) {
	case *net.TCPConn:
		c.emitError(v.SetNoDelay(false))
	case *tls.Conn:
		if netConn, ok := conn.(internal.NetConn); ok {
			c.setNoDelay(netConn.NetConn())
		}
	}
}
