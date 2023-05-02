package gws

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

type dialer struct {
	option          *ClientOption
	conn            net.Conn
	eventHandler    Event
	resp            *http.Response
	secWebsocketKey string
}

// NewClient 创建WebSocket客户端; 支持ws, wss, unix三种协议
// Create WebSocket client, support ws, wss, unix three protocols
func NewClient(handler Event, option *ClientOption) (client *Conn, resp *http.Response, e error) {
	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()

	var d = &dialer{eventHandler: handler, option: option}
	defer func() {
		if e != nil && !d.isNil(d.conn) {
			_ = d.conn.Close()
		}
	}()

	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, d.resp, err
	}

	var dialError error
	var hostname = URL.Hostname()
	var port = URL.Port()
	var host = ""
	switch URL.Scheme {
	case "ws":
		if port == "" {
			port = "80"
		}
		host = hostname + ":" + port
		d.conn, dialError = net.DialTimeout("tcp", host, option.DialTimeout)
	case "wss":
		if port == "" {
			port = "443"
		}
		host = hostname + ":" + port
		var tlsDialer = &net.Dialer{Timeout: option.DialTimeout}
		d.conn, dialError = tls.DialWithDialer(tlsDialer, "tcp", host, option.TlsConfig)
	case "unix":
		d.conn, dialError = net.DialTimeout("unix", URL.Path, option.DialTimeout)
	default:
		return nil, d.resp, internal.ErrSchema
	}

	if dialError != nil {
		return nil, d.resp, dialError
	}
	if err := d.conn.SetDeadline(time.Now().Add(option.DialTimeout)); err != nil {
		return nil, d.resp, err
	}
	return d.handshake()
}

func (c *dialer) isNil(v interface{}) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsNil()
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
	if c.resp.StatusCode != 101 {
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
