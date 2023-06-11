package gws

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lxzan/gws/internal"
	"golang.org/x/net/proxy"
)

type dialer struct {
	option          *ClientOption
	conn            net.Conn
	eventHandler    Event
	resp            *http.Response
	secWebsocketKey string
}

// NewClient 创建WebSocket客户端; 支持ws/wss
// Create WebSocket client, support ws/wss
func NewClient(handler Event, option *ClientOption) (client *Conn, resp *http.Response, e error) {
	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()

	var d = &dialer{eventHandler: handler, option: option}
	defer func() {
		if e != nil && !internal.IsNil(d.conn) {
			_ = d.conn.Close()
		}
	}()

	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, d.resp, err
	}
	if URL.Scheme != "ws" && URL.Scheme != "wss" {
		return nil, d.resp, internal.ErrSchema
	}

	var dialerInstance proxy.Dialer = &net.Dialer{Timeout: option.DialTimeout}
	if option.ProxyAddr != "" {
		proxyURL, err := url.Parse(option.ProxyAddr)
		if err != nil {
			return nil, d.resp, err
		}
		addr := proxyURL.Hostname() + ":" + proxyURL.Port()
		dialerInstance, err = proxy.SOCKS5("tcp", addr, nil, nil)
		if err != nil {
			return nil, d.resp, err
		}
	}

	port := internal.SelectValue(URL.Port() == "", internal.SelectValue(URL.Scheme == "ws", "80", "443"), URL.Port())
	hp := internal.SelectValue(URL.Hostname() == "", "127.0.0.1", URL.Hostname()) + ":" + port
	d.conn, err = dialerInstance.Dial("tcp", hp)
	if err != nil {
		return nil, d.resp, err
	}
	if URL.Scheme == "wss" {
		d.conn = tls.Client(d.conn, option.TlsConfig)
	}

	if err := d.conn.SetDeadline(time.Now().Add(option.DialTimeout)); err != nil {
		return nil, d.resp, err
	}
	return d.handshake()
}

// NewClientFromConn
func NewClientFromConn(handler Event, option *ClientOption, conn net.Conn) (client *Conn, resp *http.Response, e error) {
	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()
	d := &dialer{option: option, conn: conn, eventHandler: handler}
	defer func() {
		if e != nil && !internal.IsNil(d.conn) {
			_ = d.conn.Close()
		}
	}()
	if err := d.conn.SetDeadline(time.Now().Add(option.DialTimeout)); err != nil {
		return nil, d.resp, err
	}
	return d.handshake()
}

func (c *dialer) writeRequest() (*http.Request, error) {
	r, err := http.NewRequest(http.MethodGet, c.option.Addr, nil)
	if err != nil {
		return nil, err
	}
	r.Header = c.option.RequestHeader.Clone()
	r.Header.Set(internal.Connection.Key, internal.Connection.Val)
	r.Header.Set(internal.Upgrade.Key, internal.Upgrade.Val)
	r.Header.Set(internal.SecWebSocketVersion.Key, internal.SecWebSocketVersion.Val)
	if c.option.CompressEnabled {
		r.Header.Set(internal.SecWebSocketExtensions.Key, internal.SecWebSocketExtensions.Val)
	}
	if c.secWebsocketKey == "" {
		var key [16]byte
		binary.BigEndian.PutUint64(key[0:8], internal.AlphabetNumeric.Uint64())
		binary.BigEndian.PutUint64(key[8:16], internal.AlphabetNumeric.Uint64())
		c.secWebsocketKey = base64.StdEncoding.EncodeToString(key[0:])
		r.Header.Set(internal.SecWebSocketKey.Key, c.secWebsocketKey)
	}
	return r, r.Write(c.conn)
}

func (c *dialer) handshake() (*Conn, *http.Response, error) {
	br := bufio.NewReaderSize(c.conn, c.option.ReadBufferSize)
	request, err := c.writeRequest()
	if err != nil {
		return nil, nil, err
	}
	var channel = make(chan error)
	go func() {
		c.resp, err = http.ReadResponse(br, request)
		channel <- err
	}()
	if err := <-channel; err != nil {
		return nil, c.resp, err
	}
	if err := c.checkHeaders(); err != nil {
		return nil, c.resp, err
	}
	if err := c.conn.SetDeadline(time.Time{}); err != nil {
		return nil, c.resp, err
	}
	if err := setNoDelay(c.conn); err != nil {
		return nil, c.resp, err
	}
	var compressEnabled = c.option.CompressEnabled && strings.Contains(c.resp.Header.Get(internal.SecWebSocketExtensions.Key), "permessage-deflate")
	return serveWebSocket(false, c.option.getConfig(), new(sliceMap), c.conn, br, c.eventHandler, compressEnabled), c.resp, nil
}

func (c *dialer) checkHeaders() error {
	if c.resp.StatusCode != http.StatusSwitchingProtocols {
		return internal.ErrStatusCode
	}
	if !internal.HttpHeaderEqual(c.resp.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return internal.ErrHandshake
	}
	if !internal.HttpHeaderEqual(c.resp.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return internal.ErrHandshake
	}
	if c.resp.Header.Get(internal.SecWebSocketAccept.Key) != internal.ComputeAcceptKey(c.secWebsocketKey) {
		return internal.ErrHandshake
	}
	return nil
}
