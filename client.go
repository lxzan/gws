package gws

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lxzan/gws/internal"
)

// Dialer 拨号器接口
// Dialer interface
type Dialer interface {
	// Dial 连接到指定网络上的地址
	// Connects to the address on the named network
	Dial(network, addr string) (c net.Conn, err error)
}

type connector struct {
	option          *ClientOption
	conn            net.Conn
	eventHandler    Event
	secWebsocketKey string
}

// NewClient 创建一个新的 WebSocket 客户端连接
// Creates a new WebSocket client connection
func NewClient(handler Event, option *ClientOption) (*Conn, *http.Response, error) {
	option = initClientOption(option)
	c := &connector{option: option, eventHandler: handler}
	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, nil, err
	}
	if URL.Scheme != "ws" && URL.Scheme != "wss" {
		return nil, nil, ErrUnsupportedProtocol
	}

	var tlsEnabled = URL.Scheme == "wss"
	dialer, err := option.NewDialer()
	if err != nil {
		return nil, nil, err
	}

	c.conn, err = dialer.Dial("tcp", internal.GetAddrFromURL(URL, tlsEnabled))
	if err != nil {
		return nil, nil, err
	}
	if tlsEnabled {
		if option.TlsConfig == nil {
			option.TlsConfig = &tls.Config{}
		}
		if option.TlsConfig.ServerName == "" {
			option.TlsConfig.ServerName = URL.Hostname()
		}
		c.conn = tls.Client(c.conn, option.TlsConfig)
	}

	client, resp, err := c.handshake()
	if err != nil {
		_ = c.conn.Close()
	}
	return client, resp, err
}

// NewClientFromConn 通过外部连接创建客户端, 支持 TCP/KCP/Unix Domain Socket
// Create new client via external connection, supports TCP/KCP/Unix Domain Socket.
func NewClientFromConn(handler Event, option *ClientOption, conn net.Conn) (*Conn, *http.Response, error) {
	option = initClientOption(option)
	c := &connector{option: option, conn: conn, eventHandler: handler}
	client, resp, err := c.handshake()
	if err != nil {
		_ = c.conn.Close()
	}
	return client, resp, err
}

// 发送HTTP请求, 即WebSocket握手
// Sends an http request, i.e., websocket handshake
func (c *connector) request() (*http.Response, *bufio.Reader, error) {
	_ = c.conn.SetDeadline(time.Now().Add(c.option.HandshakeTimeout))
	ctx, cancel := context.WithTimeout(context.Background(), c.option.HandshakeTimeout)
	defer cancel()

	// 构建HTTP请求
	// building a http request
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.option.Addr, nil)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range c.option.RequestHeader {
		if k == "Host" && len(v) > 0 {
			r.Host = v[0]
		}
		r.Header[k] = v
	}
	r.Header.Set(internal.Connection.Key, internal.Connection.Val)
	r.Header.Set(internal.Upgrade.Key, internal.Upgrade.Val)
	r.Header.Set(internal.SecWebSocketVersion.Key, internal.SecWebSocketVersion.Val)
	if c.option.PermessageDeflate.Enabled {
		r.Header.Set(internal.SecWebSocketExtensions.Key, c.option.PermessageDeflate.genRequestHeader())
	}
	if c.secWebsocketKey == "" {
		var key [16]byte
		binary.BigEndian.PutUint64(key[0:8], internal.AlphabetNumeric.Uint64())
		binary.BigEndian.PutUint64(key[8:16], internal.AlphabetNumeric.Uint64())
		c.secWebsocketKey = base64.StdEncoding.EncodeToString(key[0:])
		r.Header.Set(internal.SecWebSocketKey.Key, c.secWebsocketKey)
	}

	var ch = make(chan error)

	// 发送http请求
	// send http request
	go func() { ch <- r.Write(c.conn) }()

	// 同步等待请求是否发送成功
	// Synchronized waiting for the request to be sent successfully
	select {
	case err = <-ch:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		return nil, nil, err
	}

	// 读取响应结果
	// Read the response result
	br := bufio.NewReaderSize(c.conn, c.option.ReadBufferSize)
	resp, err := http.ReadResponse(br, r)
	return resp, br, err
}

// 获取压缩拓展结果
// Get compression expansion results
func (c *connector) getPermessageDeflate(extensions string) PermessageDeflate {
	serverPD := permessageNegotiation(extensions)
	clientPD := c.option.PermessageDeflate
	pd := PermessageDeflate{
		Enabled:               clientPD.Enabled && strings.Contains(extensions, internal.PermessageDeflate),
		Threshold:             clientPD.Threshold,
		Level:                 clientPD.Level,
		PoolSize:              clientPD.PoolSize,
		ServerContextTakeover: serverPD.ServerContextTakeover,
		ClientContextTakeover: serverPD.ClientContextTakeover,
		ServerMaxWindowBits:   serverPD.ServerMaxWindowBits,
		ClientMaxWindowBits:   serverPD.ClientMaxWindowBits,
	}
	pd.setThreshold(false)
	return pd
}

// 执行 WebSocket 握手操作
// Performs the WebSocket handshake operation
func (c *connector) handshake() (*Conn, *http.Response, error) {
	resp, br, err := c.request()
	if err != nil {
		return nil, resp, err
	}
	if err = c.checkHeaders(resp); err != nil {
		return nil, resp, err
	}
	subprotocol, err := c.getSubProtocol(resp)
	if err != nil {
		return nil, resp, err
	}

	var extensions = resp.Header.Get(internal.SecWebSocketExtensions.Key)
	var pd = c.getPermessageDeflate(extensions)
	socket := &Conn{
		ss:                c.option.NewSession(),
		isServer:          false,
		subprotocol:       subprotocol,
		pd:                pd,
		conn:              c.conn,
		config:            c.option.getConfig(),
		br:                br,
		continuationFrame: continuationFrame{},
		fh:                frameHeader{},
		handler:           c.eventHandler,
		closed:            0,
		deflater:          new(deflater),
		writeQueue:        workerQueue{maxConcurrency: 1},
		readQueue:         make(channel, c.option.ParallelGolimit),
	}

	// 压缩字典和解压字典内存开销比较大, 故使用懒加载
	// Compressing and decompressing dictionaries has a large memory overhead, so use lazy loading.
	if pd.Enabled {
		socket.deflater.initialize(false, pd, c.option.ReadMaxPayloadSize)
		if pd.ServerContextTakeover {
			socket.dpsWindow.initialize(nil, pd.ServerMaxWindowBits)
		}
		if pd.ClientContextTakeover {
			socket.cpsWindow.initialize(nil, pd.ClientMaxWindowBits)
		}
	}
	return socket, resp, c.conn.SetDeadline(time.Time{})
}

// 从响应中获取子协议
// Retrieves the subprotocol from the response
func (c *connector) getSubProtocol(resp *http.Response) (string, error) {
	a := internal.Split(c.option.RequestHeader.Get(internal.SecWebSocketProtocol.Key), ",")
	b := internal.Split(resp.Header.Get(internal.SecWebSocketProtocol.Key), ",")
	subprotocol := internal.GetIntersectionElem(a, b)
	if len(a) > 0 && subprotocol == "" {
		return "", ErrSubprotocolNegotiation
	}
	return subprotocol, nil
}

// 检查响应头以验证握手是否成功
// Checks the response headers to verify if the handshake was successful
func (c *connector) checkHeaders(resp *http.Response) error {
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if !internal.HttpHeaderContains(resp.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return fmt.Errorf("missing %s header", internal.Connection.Key)
	}
	if !strings.EqualFold(resp.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return fmt.Errorf("missing %s header", internal.Upgrade.Key)
	}
	if resp.Header.Get(internal.SecWebSocketAccept.Key) != internal.ComputeAcceptKey(c.secWebsocketKey) {
		return fmt.Errorf("invalid %s header", internal.SecWebSocketAccept.Key)
	}
	return nil
}
