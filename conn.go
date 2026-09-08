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

// Conn WebSocket连接
type Conn struct {
	mu                sync.Mutex        // 互斥锁
	ss                SessionStorage    // 会话存储
	ev                atomic.Value      // 错误值
	isServer          bool              // 是否为服务器端
	subprotocol       string            // 子协议
	conn              net.Conn          // 底层网络连接
	config            *Config           // 配置
	br                *bufio.Reader     // 缓冲读取器
	continuationFrame continuationFrame // 持续帧
	fh                frameHeader       // 帧头
	handler           Event             // 事件处理器
	closed            uint32            // 关闭状态
	readQueue         channel           // 读取队列
	writeQueue        workerQueue       // 写入队列
	deflater          *deflater         // 压缩器
	dpsWindow         slideWindow       // 解压字典滑动窗口
	cpsWindow         slideWindow       // 压缩字典滑动窗口
	pd                PermessageDeflate // 压缩扩展配置

	// NextReader 当前活跃的 reader, 用于在下一次 NextReader 或错误时释放资源.
	rr nextMessageReader

	// NextReader 复用的 reader 实例, 首次调用 NextReader 时惰性分配.
	urr *uncompressedMessageReader
	crr *messageReader

	// NextReader 代数, 每次返回新 reader 时递增, 用于检测过期的 reader 句柄.
	readGen uint32

	// 掩码键随机源状态, 仅客户端连接使用, 通过原子操作访问, 零值安全.
	// 使用 atomic.Uint64 保证字段在32位平台上也是8字节对齐, 避免原子操作恐慌.
	randState atomic.Uint64
}

// maskKeyStep 掩码键随机源的推进步长, 取黄金分割比的64位表示.
// 步长为奇数, 状态序列可遍历全部2^64个值, 配合 splitmix64 混合输出无退化.
const maskKeyStep = 0x9E3779B97F4A7C15

// maskKeyShift 混合结果的右移位数, 取64位输出的高32位作为掩码键.
const maskKeyShift = 32

// nextMaskKey 生成一个随机掩码键.
// 状态推进与种子化均通过原子操作完成, 无需持有连接锁.
// 服务端连接不掩码帧, 直接返回0, 不付出随机数开销.
func (c *Conn) nextMaskKey() uint32 {
	if c.isServer {
		return 0
	}
	return uint32(internal.SplitMix64(c.randState.Add(maskKeyStep)) >> maskKeyShift)
}

// ReadLoop 循环读取消息.
// 如果复用了HTTP Server, 建议开启goroutine, 阻塞会导致请求上下文无法被GC.
func (c *Conn) ReadLoop() {
	if err := c.dispatchOpen(); err != nil {
		c.handleReadError(err)
		return
	}

	for {
		if err := c.readMessage(); err != nil {
			c.handleReadError(err)
			break
		}
	}
}

// ReadMessage 读取并返回单个完整的 WebSocket 消息.
// 不会触发 OnOpen 事件, 也不会派发给 OnMessage 回调.
// 如果发生错误, 会触发错误事件并进行资源回收, 和 ReadLoop 结束时的处理逻辑一致.
func (c *Conn) ReadMessage() (*Message, error) {
	if c.isClosed() {
		return nil, ErrConnClosed
	}
	for {
		msg, err := c.readFrame()
		if err == nil && msg != nil {
			err = c.processMessage(msg)
		}
		if err != nil {
			if msg != nil {
				_ = msg.Close()
			}
			c.handleReadError(err)
			return nil, err
		}
		if msg != nil {
			return msg, nil
		}
	}
}

// handleReadError 处理读取错误: 触发错误事件, 分发关闭回调并回收资源.
func (c *Conn) handleReadError(err error) {
	c.emitError(true, err)

	evErr, ok := c.ev.Load().(error)
	_ = c.dispatchControl(OpcodeCloseConnection, nil, internal.SelectValue(ok, evErr, errEmpty))

	c.closeNextReader()

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

// isClosed 检查连接是否已关闭
func (c *Conn) isClosed() bool {
	return atomic.LoadUint32(&c.closed) == 1
}

// emitError 处理错误事件
func (c *Conn) emitError(reading bool, err error) {
	if err == nil {
		return
	}

	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		var sendCode, sendErr = internal.CloseGoingAway, error(internal.CloseGoingAway)
		if reading {
			switch v := err.(type) {
			case internal.StatusCode:
				sendCode, sendErr = v, v
			case *internal.Error:
				sendCode, sendErr, err = v.Code, v.Err, v.Err
			default:
				sendCode, sendErr = internal.CloseNormalClosure, err
			}
		}

		var reason = append(sendCode.Bytes(), sendErr.Error()...)
		_ = c.writeClose(err, reason)
	}
}

// emitClose 处理关闭事件
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
		if !internal.CheckEncoding(c.config.CheckUtf8Enabled, uint8(OpcodeCloseConnection), buf.Bytes()) {
			responseCode = internal.CloseUnsupportedData
		}
	}
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		_ = c.writeClose(&CloseError{Code: realCode, Reason: buf.Bytes()}, responseCode.Bytes())
	}
	return internal.CloseNormalClosure
}

// SetDeadline 设置连接的截止时间
func (c *Conn) SetDeadline(t time.Time) error {
	err := c.conn.SetDeadline(t)
	c.emitError(false, err)
	return err
}

// SetReadDeadline 设置读取操作的截止时间
func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.conn.SetReadDeadline(t)
	c.emitError(false, err)
	return err
}

// SetWriteDeadline 设置写入操作的截止时间
func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.conn.SetWriteDeadline(t)
	c.emitError(false, err)
	return err
}

// LocalAddr 返回本地网络地址
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr 返回远程网络地址
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// NetConn 获取底层的网络连接
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// SetNoDelay 设置无延迟模式.
// 控制操作系统是否应该延迟数据包传输以期望发送更少的数据包(Nagle算法).
// 默认值是 true（无延迟），这意味着数据在 Write 之后尽快发送.
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
func (c *Conn) SubProtocol() string { return c.subprotocol }

// Session 获取会话存储
func (c *Conn) Session() SessionStorage { return c.ss }
