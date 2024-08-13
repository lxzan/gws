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

	"github.com/lxzan/gws/internal"
)

// Conn 结构体表示一个 WebSocket 连接
// Conn struct represents a WebSocket connection
type Conn struct {
	// 互斥锁，用于保护共享资源
	// Mutex to protect shared resources
	mu sync.Mutex

	// 会话存储，用于存储会话数据
	// Session storage for storing session data
	ss SessionStorage

	// 用于存储错误的原子值
	// Atomic value for storing errors
	err atomic.Value

	// 标识是否为服务器端
	// Indicates if this is a server-side connection
	isServer bool

	// 子协议
	// Subprotocol
	subprotocol string

	// 底层网络连接
	// Underlying network connection
	conn net.Conn

	// 配置信息
	// Configuration information
	config *Config

	// 缓冲读取器
	// Buffered reader
	br *bufio.Reader

	// 持续帧
	// Continuation frame
	continuationFrame continuationFrame

	// 帧头
	// Frame header
	fh frameHeader

	// 事件处理器
	// Event handler
	handler Event

	// 关闭状态
	// Closed state
	closed uint32

	// 读取队列
	// Read queue
	readQueue channel

	// 写入队列
	// Write queue
	writeQueue workerQueue

	// 压缩器
	// Deflater
	deflater *deflater

	// 数据包发送窗口
	// Data packet send window
	dpsWindow slideWindow

	// 数据包接收窗口
	// Data packet receive window
	cpsWindow slideWindow

	// 每消息压缩
	// Per-message deflate
	pd PermessageDeflate
}

// ReadLoop
// 循环读取消息. 如果复用了HTTP Server, 建议开启goroutine, 阻塞会导致请求上下文无法被GC.
// Read messages in a loop.
// If HTTP Server is reused, it is recommended to enable goroutine, as blocking will prevent the context from being GC.
func (c *Conn) ReadLoop() {
	c.handler.OnOpen(c)

	// 无限循环读取消息, 如果发生错误则触发错误事件并退出循环
	// Infinite loop to read messages, if an error occurs, trigger the error event and exit the loop
	for {
		if err := c.readMessage(); err != nil {
			c.emitError(err)
			break
		}
	}

	err, ok := c.err.Load().(error)
	c.handler.OnClose(c, internal.SelectValue(ok, err, errEmpty))

	// 回收资源
	// Reclaim resources
	if c.isServer {
		c.br.Reset(nil)
		c.config.brPool.Put(c.br)
		c.br = nil
		if c.cpsWindow.enabled {
			c.config.cswPool.Put(c.cpsWindow.dict)
			c.cpsWindow.dict = nil
		}
		if c.dpsWindow.enabled {
			c.config.dswPool.Put(c.dpsWindow.dict)
			c.dpsWindow.dict = nil
		}
	}
}

// 获取压缩字典
// Get compressed dictionary
func (c *Conn) getCpsDict(isBroadcast bool) []byte {
	// 广播模式必须保证每一帧都是相同的内容, 所以不使用上下文接管优化压缩率
	// In broadcast mode, each frame must be the same content, so context takeover is not used to optimize compression ratio
	if isBroadcast {
		return nil
	}

	// 如果是服务器并且服务器上下文接管启用，返回压缩字典
	// If it is a server and server context takeover is enabled, return the compression dictionary
	if c.isServer && c.pd.ServerContextTakeover {
		return c.cpsWindow.dict
	}

	// 如果是客户端并且客户端上下文接管启用，返回压缩字典
	// If client-side and client context takeover is enabled, return the compression dictionary
	if !c.isServer && c.pd.ClientContextTakeover {
		return c.cpsWindow.dict
	}

	return nil
}

// 获取解压字典
// Get decompression dictionary
func (c *Conn) getDpsDict() []byte {
	// 如果是服务器并且客户端上下文接管启用，返回解压字典
	// If it is a server and client context takeover is enabled, return the decompression dictionary
	if c.isServer && c.pd.ClientContextTakeover {
		return c.dpsWindow.dict
	}

	// 如果是客户端并且服务器上下文接管启用，返回解压字典
	// If it is a client and server context takeover is enabled, return the decompressed dictionary
	if !c.isServer && c.pd.ServerContextTakeover {
		return c.dpsWindow.dict
	}

	return nil
}

// UTF8编码检查
// UTF8 encoding check
func (c *Conn) isTextValid(opcode Opcode, payload []byte) bool {
	if c.config.CheckUtf8Enabled {
		return internal.CheckEncoding(uint8(opcode), payload)
	}
	return true
}

// 检查连接是否已关闭
// Checks if the connection is closed
func (c *Conn) isClosed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}

// 关闭连接并存储错误信息
// Closes the connection and stores the error information
func (c *Conn) close(reason []byte, err error) {
	c.err.Store(err)
	_ = c.doWrite(OpcodeCloseConnection, internal.Bytes(reason))
	_ = c.conn.Close()
}

// 处理错误事件
// Handle the error event
func (c *Conn) emitError(err error) {
	if err == nil {
		return
	}

	// 使用原子操作检查并设置连接的关闭状态
	// Use atomic operation to check and set the closed state of the connection
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		var responseCode = internal.CloseNormalClosure
		var responseErr error = internal.CloseNormalClosure

		// 根据错误类型设置响应代码和响应错误
		// Set response code and response error based on the error type
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

		// 如果内容长度超过阈值，截断内容
		// If the content length exceeds the threshold, truncate the content
		if len(content) > internal.ThresholdV1 {
			content = content[:internal.ThresholdV1]
		}

		c.close(content, responseErr)
	}
}

// 处理关闭事件
// Handles the close event
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

// SetDeadline 设置连接的截止时间
// Sets the deadline for the connection
func (c *Conn) SetDeadline(t time.Time) error {
	err := c.conn.SetDeadline(t)
	c.emitError(err)
	return err
}

// SetReadDeadline 设置读取操作的截止时间
// Sets the deadline for read operations
func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.conn.SetReadDeadline(t)
	c.emitError(err)
	return err
}

// SetWriteDeadline 设置写入操作的截止时间
// Sets the deadline for write operations
func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.conn.SetWriteDeadline(t)
	c.emitError(err)
	return err
}

// LocalAddr 返回本地网络地址
// Returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr 返回远程网络地址
// Returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// NetConn
// 获取底层的 TCP/TLS/KCP 等连接
// Gets the underlying TCP/TLS/KCP... connection
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// SetNoDelay
// 控制操作系统是否应该延迟数据包传输以期望发送更少的数据包（Nagle 算法）。
// 默认值是 true（无延迟），这意味着数据在 Write 之后尽快发送。
// Controls whether the operating system should delay packet transmission in hopes of sending fewer packets (Nagle's algorithm).
// The default is true (no delay), meaning that data is sent as soon as possible after a Write.
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
// Gets the negotiated sub-protocol
func (c *Conn) SubProtocol() string { return c.subprotocol }

// Session 获取会话存储
// Gets the session storage
func (c *Conn) Session() SessionStorage { return c.ss }
