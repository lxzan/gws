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
)

type connector struct {
	option          *ClientOption
	conn            net.Conn
	eventHandler    Event
	resp            *http.Response
	secWebsocketKey string
}

// NewClient 创建WebSocket客户端; 支持ws/wss
// Create WebSocket client, support ws/wss
func NewClient(handler Event, option *ClientOption) (*Conn, *http.Response, error) {
	option = initClientOption(option)
	c := &connector{option: option, eventHandler: handler, resp: &http.Response{}}
	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, nil, err
	}
	if URL.Scheme != "ws" && URL.Scheme != "wss" {
		return nil, nil, internal.ErrSchema
	}

	var tlsEnabled = URL.Scheme == "wss"
	dialer, err := option.NewDialer()
	if err != nil {
		return nil, nil, err
	}

	port := internal.SelectValue(URL.Port() == "", internal.SelectValue(tlsEnabled, "443", "80"), URL.Port())
	hp := internal.SelectValue(URL.Hostname() == "", "127.0.0.1", URL.Hostname()) + ":" + port
	c.conn, err = dialer.Dial("tcp", hp)
	if err != nil {
		return nil, nil, err
	}
	if tlsEnabled {
		c.conn = tls.Client(c.conn, option.TlsConfig)
	}

	client, resp, err := c.handshake()
	if err != nil {
		_ = c.conn.Close()
	}
	return client, resp, err
}

// NewClientFromConn
func NewClientFromConn(handler Event, option *ClientOption, conn net.Conn) (*Conn, *http.Response, error) {
	option = initClientOption(option)
	c := &connector{option: option, conn: conn, eventHandler: handler, resp: &http.Response{}}
	client, resp, err := c.handshake()
	if err != nil {
		_ = c.conn.Close()
	}
	return client, resp, err
}

func (c *connector) writeRequest() (*http.Request, error) {
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

func (c *connector) handshake() (*Conn, *http.Response, error) {
	if err := c.conn.SetDeadline(time.Now().Add(c.option.HandshakeTimeout)); err != nil {
		return nil, c.resp, err
	}
	br := bufio.NewReaderSize(c.conn, c.option.ReadBufferSize)
	request, err := c.writeRequest()
	if err != nil {
		return nil, c.resp, err
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

func (c *connector) checkHeaders() error {
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
