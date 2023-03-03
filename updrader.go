package gws

import (
	"compress/flate"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultReadAsyncGoLimit     = 8
	defaultReadAsyncCap         = 256
	defaultWriteAsyncCap        = 256
	defaultCompressLevel        = flate.BestSpeed
	defaultReadMaxPayloadSize   = 16 * 1024 * 1024
	defaultWriteMaxPayloadSize  = 16 * 1024 * 1024
	defaultCompressionThreshold = 512
)

type (
	Request struct {
		*http.Request                 // http request
		SessionStorage SessionStorage // store user session
	}

	// Upgrader websocket upgrader
	Config struct {
		// 是否开启异步读, 开启的话会并行调用OnMessage
		ReadAsyncEnabled bool

		// 异步读的最大并行协程数量
		ReadAsyncGoLimit int

		// 异步读的容量限制, 容量溢出将会返回错误
		ReadAsyncCap int

		// 最大读取的消息内容长度
		ReadMaxPayloadSize int

		// 异步写的容量限制, 容量溢出将会返回错误
		WriteAsyncCap int

		// 最大写入的消息内容长度
		WriteMaxPayloadSize int

		// 是否开启数据压缩
		CompressEnabled bool

		// 压缩级别
		CompressLevel int

		// 压缩阈值, 低于阈值的消息不会被压缩
		CompressionThreshold int

		// 是否检查文本utf8编码, 关闭性能会好点
		CheckUtf8Enabled bool
	}

	ServerOption struct {
		ReadAsyncEnabled     bool
		ReadAsyncGoLimit     int
		ReadAsyncCap         int
		ReadMaxPayloadSize   int
		WriteAsyncCap        int
		WriteMaxPayloadSize  int
		CompressEnabled      bool
		CompressLevel        int
		CompressionThreshold int
		CheckUtf8Enabled     bool

		// 连接握手时添加的额外的响应头, 如果客户端不支持就不要传
		// https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3
		// attention: client may not support custom response header, use nil instead
		ResponseHeader http.Header

		// 检查请求来源
		// Check the origin of the request
		CheckOrigin func(r *Request) bool
	}

	Upgrader struct {
		option       *ServerOption
		config       *Config
		eventHandler Event
	}
)

// Initialize the upgrader configure
// 如果没有使用NewUpgrader, 需要调用此方法初始化配置
func (c *Upgrader) Initialize() *Upgrader {
	if c.option.ReadMaxPayloadSize <= 0 {
		c.option.ReadMaxPayloadSize = defaultReadMaxPayloadSize
	}
	if c.option.ReadAsyncGoLimit <= 0 {
		c.option.ReadAsyncGoLimit = defaultReadAsyncGoLimit
	}
	if c.option.ReadAsyncCap <= 0 {
		c.option.ReadAsyncCap = defaultReadAsyncCap
	}
	if c.option.WriteAsyncCap <= 0 {
		c.option.WriteAsyncCap = defaultWriteAsyncCap
	}
	if c.option.WriteMaxPayloadSize <= 0 {
		c.option.WriteMaxPayloadSize = defaultWriteMaxPayloadSize
	}
	if c.option.CompressEnabled && c.option.CompressLevel == 0 {
		c.option.CompressLevel = defaultCompressLevel
	}
	if c.option.CompressionThreshold <= 0 {
		c.option.CompressionThreshold = defaultCompressionThreshold
	}
	if c.option.CheckOrigin == nil {
		c.option.CheckOrigin = func(r *Request) bool {
			return true
		}
	}
	if c.option.ResponseHeader == nil {
		c.option.ResponseHeader = http.Header{}
	}

	c.config = &Config{
		ReadAsyncEnabled:     c.option.ReadAsyncEnabled,
		ReadAsyncGoLimit:     c.option.ReadAsyncGoLimit,
		ReadAsyncCap:         c.option.ReadAsyncCap,
		ReadMaxPayloadSize:   c.option.ReadMaxPayloadSize,
		WriteAsyncCap:        c.option.WriteAsyncCap,
		WriteMaxPayloadSize:  c.option.WriteMaxPayloadSize,
		CompressEnabled:      c.option.CompressEnabled,
		CompressLevel:        c.option.CompressLevel,
		CompressionThreshold: c.option.CompressionThreshold,
		CheckUtf8Enabled:     c.option.CheckUtf8Enabled,
	}
	return c
}

func NewUpgrader(eventHandler Event, option *ServerOption) *Upgrader {
	if option == nil {
		option = new(ServerOption)
	}
	var u = &Upgrader{
		option:       option,
		eventHandler: eventHandler,
	}
	return u.Initialize()
}

func (c *Upgrader) connectHandshake(conn net.Conn, headers http.Header, websocketKey string) error {
	var buf = make([]byte, 0, 256)
	buf = append(buf, "HTTP/1.1 101 Switching Protocols\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Sec-WebSocket-Accept: "...)
	buf = append(buf, internal.ComputeAcceptKey(websocketKey)...)
	buf = append(buf, "\r\n"...)
	for k, _ := range headers {
		buf = append(buf, k...)
		buf = append(buf, ": "...)
		buf = append(buf, headers.Get(k)...)
		buf = append(buf, "\r\n"...)
	}
	buf = append(buf, "\r\n"...)
	_, err := conn.Write(buf)
	return err
}

// Accept http upgrade to websocket protocol
func (c *Upgrader) Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	socket, err := c.doAccept(w, r)
	if err != nil {
		if socket != nil && socket.conn != nil {
			_ = socket.conn.Close()
		}
		return nil, err
	}
	return socket, err
}

func (c *Upgrader) doAccept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	var request = &Request{Request: r, SessionStorage: &sliceMap{}}
	var header = c.option.ResponseHeader.Clone()
	if !c.option.CheckOrigin(request) {
		return nil, internal.ErrCheckOrigin
	}

	var compressEnabled = false
	if r.Method != http.MethodGet {
		return nil, internal.ErrGetMethodRequired
	}
	if version := r.Header.Get(internal.SecWebSocketVersion.Key); version != internal.SecWebSocketVersion.Val {
		msg := "websocket protocol not supported: " + version
		return nil, errors.New(msg)
	}
	if val := r.Header.Get(internal.Connection.Key); strings.ToLower(val) != strings.ToLower(internal.Connection.Val) {
		return nil, internal.ErrHandshake
	}
	if val := r.Header.Get(internal.Upgrade.Key); strings.ToLower(val) != internal.Upgrade.Val {
		return nil, internal.ErrHandshake
	}
	if val := r.Header.Get(internal.SecWebSocketExtensions.Key); strings.Contains(val, "permessage-deflate") && c.config.CompressEnabled {
		header.Set(internal.SecWebSocketExtensions.Key, internal.SecWebSocketExtensions.Val)
		compressEnabled = true
	}
	var websocketKey = r.Header.Get(internal.SecWebSocketKey.Key)
	if websocketKey == "" {
		return nil, internal.ErrHandshake
	}

	// Hijack
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, internal.CloseInternalServerErr
	}
	netConn, brw, err := hj.Hijack()
	if err != nil {
		return &Conn{conn: netConn}, err
	}
	if err := c.connectHandshake(netConn, header, websocketKey); err != nil {
		return &Conn{conn: netConn}, err
	}

	if err := internal.Errors(
		func() error { return netConn.SetDeadline(time.Time{}) },
		func() error { return netConn.SetReadDeadline(time.Time{}) },
		func() error { return netConn.SetWriteDeadline(time.Time{}) },
		func() error { return setNoDelay(netConn) }); err != nil {
		return nil, err
	}
	return serveWebSocket(c.config, request, netConn, brw, c.eventHandler, compressEnabled), nil
}
