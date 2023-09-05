package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/lxzan/gws/internal"
)

type Conn struct {
	SessionStorage    SessionStorage    // 会话
	err               atomic.Value      // 错误
	isServer          bool              // 是否为服务器
	subprotocol       string            // 子协议
	conn              net.Conn          // 底层连接
	config            *Config           // 配置
	br                *bufio.Reader     // 读缓存
	continuationFrame continuationFrame // 连续帧
	fh                frameHeader       // 帧头
	handler           Event             // 事件处理器
	closed            uint32            // 是否关闭
	readQueue         channel           // 消息处理队列
	writeQueue        workerQueue       // 发送队列
	compressEnabled   bool              // 是否压缩
	compressor        *compressor       // 压缩器
	decompressor      *decompressor     // 解压器
}

func (c *Conn) init() *Conn {
	c.writeQueue = workerQueue{maxConcurrency: 1}
	if c.config.ReadAsyncEnabled {
		c.readQueue = make(channel, c.config.ReadAsyncGoLimit)
	}
	if c.compressEnabled {
		c.compressor = c.config.compressors.Select()
		c.decompressor = c.config.decompressors.Select()
	}
	return c
}

// ReadLoop 循环读取消息. 如果复用了HTTP Server, 建议开启goroutine, 阻塞会导致请求上下文无法被GC.
// Read messages in a loop.
// If HTTP Server is reused, it is recommended to enable goroutine, as blocking will prevent the context from being GC.
func (c *Conn) ReadLoop() {
	c.handler.OnOpen(c)
	for {
		if err := c.readMessage(); err != nil {
			c.emitError(err)
			break
		}
	}
	c.handler.OnClose(c, c.err.Load().(error))
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

func (c *Conn) isClosed() bool { return atomic.LoadUint32(&c.closed) == 1 }

func (c *Conn) close(reason []byte, err error) {
	c.err.Store(err)
	_ = c.doWrite(OpcodeCloseConnection, reason)
	_ = c.conn.Close()
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
		c.close(content, responseErr)
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
		c.close(responseCode.Bytes(), &CloseError{Code: realCode, Reason: buf.Bytes()})
	}
	return internal.CloseNormalClosure
}

// SetDeadline sets deadline
func (c *Conn) SetDeadline(t time.Time) error {
	if c.isClosed() {
		return ErrConnClosed
	}
	err := c.conn.SetDeadline(t)
	c.emitError(err)
	return err
}

// SetReadDeadline sets read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	if c.isClosed() {
		return ErrConnClosed
	}
	err := c.conn.SetReadDeadline(t)
	c.emitError(err)
	return err
}

// SetWriteDeadline sets write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if c.isClosed() {
		return ErrConnClosed
	}
	err := c.conn.SetWriteDeadline(t)
	c.emitError(err)
	return err
}

func (c *Conn) LocalAddr() net.Addr { return c.conn.LocalAddr() }

func (c *Conn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// NetConn get tcp/tls/kcp... connection
func (c *Conn) NetConn() net.Conn { return c.conn }

// SetNoDelay controls whether the operating system should delay
// packet transmission in hopes of sending fewer packets (Nagle's
// algorithm).  The default is true (no delay), meaning that data is
// sent as soon as possible after a Write.
func (c *Conn) SetNoDelay(noDelay bool) error {
	switch v := c.conn.(type) {
	case *net.TCPConn:
		return v.SetNoDelay(noDelay)
	case *tls.Conn:
		if netConn, ok := v.NetConn().(*net.TCPConn); ok {
			return netConn.SetNoDelay(noDelay)
		}
	}
	return nil
}

// SubProtocol 获取协商的子协议
// Get negotiated sub-protocols
func (c *Conn) SubProtocol() string { return c.subprotocol }
