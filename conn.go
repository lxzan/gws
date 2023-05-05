package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/lxzan/gws/internal"
)

type Conn struct {
	// store session information
	SessionStorage SessionStorage
	// is server
	isServer bool
	// whether to use compression
	compressEnabled bool
	// tcp connection
	conn net.Conn
	// server configs
	config *Config
	// read buffer
	rbuf *bufio.Reader
	// continuation frame
	continuationFrame continuationFrame
	// frame header for read
	fh frameHeader
	// WebSocket Event Handler
	handler Event

	// whether server is closed
	closed uint32
	// write lock
	wmu sync.Mutex
	// async read task queue
	readQueue workerQueue
	// async write task queue
	writeQueue workerQueue
}

func serveWebSocket(isServer bool, config *Config, session SessionStorage, netConn net.Conn, br *bufio.Reader, handler Event, compressEnabled bool) *Conn {
	c := &Conn{
		isServer:        isServer,
		SessionStorage:  session,
		config:          config,
		compressEnabled: compressEnabled,
		conn:            netConn,
		closed:          0,
		wmu:             sync.Mutex{},
		rbuf:            br,
		fh:              frameHeader{},
		handler:         handler,
		readQueue:       workerQueue{maxConcurrency: int32(config.ReadAsyncGoLimit), capacity: config.ReadAsyncCap},
		writeQueue:      workerQueue{maxConcurrency: 1, capacity: config.WriteAsyncCap},
	}
	return c
}

// Listen 监听websocket消息
// Deprecated: Listen will be deprecated in future versions, please use ReadLoop instead.
func (c *Conn) Listen() {
	c.ReadLoop()
}

// ReadLoop start a read message loop
// 启动一个读消息的死循环
func (c *Conn) ReadLoop() {
	defer c.conn.Close()

	c.handler.OnOpen(c)

	for {
		if err := c.readMessage(); err != nil {
			c.emitError(err)
			return
		}
	}
}

func (c *Conn) isTextValid(opcode Opcode, payload []byte) bool {
	if !c.config.CheckUtf8Enabled {
		return true
	}
	switch opcode {
	case OpcodeText, OpcodeCloseConnection:
		return utf8.Valid(payload)
	default:
		return true
	}
}

func (c *Conn) isClosed() bool {
	return atomic.LoadUint32(&c.closed) == 1
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
		_ = c.conn.SetDeadline(time.Now())
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
		if !c.isTextValid(OpcodeCloseConnection, buf.Bytes()) {
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
func (c *Conn) SetDeadline(t time.Time) error {
	if c.isClosed() {
		return nil
	}
	err := c.conn.SetDeadline(t)
	c.emitError(err)
	return err
}

// SetReadDeadline sets read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	if c.isClosed() {
		return nil
	}
	err := c.conn.SetReadDeadline(t)
	c.emitError(err)
	return err
}

// SetWriteDeadline sets write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if c.isClosed() {
		return nil
	}
	err := c.conn.SetWriteDeadline(t)
	c.emitError(err)
	return err
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
func setNoDelay(conn net.Conn) error {
	switch v := conn.(type) {
	case *net.TCPConn:
		return v.SetNoDelay(false)
	case *tls.Conn:
		if netConn, ok := conn.(internal.NetConn); ok {
			return setNoDelay(netConn.NetConn())
		}
	}
	return nil
}
