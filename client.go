package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewClient 创建WebSocket客户端
func NewClient(handler Event, option *ClientOption) (client *Conn, resp *http.Response, e error) {
	var d = &dialer{eventHandler: handler, resp: &http.Response{}}
	defer func() {
		if e != nil && d.conn != nil {
			_ = d.conn.Close()
		}
	}()

	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()

	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, d.resp, err
	}

	var conn net.Conn
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
		conn, dialError = net.DialTimeout("tcp", host, option.DialTimeout)
	case "wss":
		if port == "" {
			port = "443"
		}
		host = hostname + ":" + port
		var tlsDialer = &net.Dialer{Timeout: option.DialTimeout}
		conn, dialError = tls.DialWithDialer(tlsDialer, "tcp", host, option.TlsConfig)
	default:
		return nil, d.resp, internal.ErrSchema
	}

	if dialError != nil {
		return nil, d.resp, dialError
	}
	if err := conn.SetDeadline(time.Now().Add(option.DialTimeout)); err != nil {
		return nil, d.resp, err
	}

	d.host = host
	d.option = option
	d.conn = conn
	return d.handshake()
}

type dialer struct {
	option       *ClientOption
	conn         net.Conn
	host         string
	eventHandler Event
	resp         *http.Response
}

func (c *dialer) stradd(ss ...string) string {
	var b []byte
	for _, item := range ss {
		b = append(b, item...)
	}
	return string(b)
}

// 生成报文
func (c *dialer) generateTelegram() []byte {
	if c.option.RequestHeader.Get(internal.SecWebSocketKey.Key) == "" {
		var key [16]byte
		binary.BigEndian.PutUint64(key[0:8], internal.AlphabetNumeric.Uint64())
		binary.BigEndian.PutUint64(key[8:16], internal.AlphabetNumeric.Uint64())
		c.option.RequestHeader.Set(internal.SecWebSocketKey.Key, base64.StdEncoding.EncodeToString(key[0:]))
	}
	if c.option.CompressEnabled {
		c.option.RequestHeader.Set(internal.SecWebSocketExtensions.Key, internal.SecWebSocketExtensions.Val)
	}

	var buf = make([]byte, 0, 256)
	buf = append(buf, c.stradd("GET ", c.option.Addr, " HTTP/1.1\r\n")...)
	buf = append(buf, c.stradd("Host: ", c.host, "\r\n")...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Sec-WebSocket-Version: 13\r\n"...)
	for k, _ := range c.option.RequestHeader {
		buf = append(buf, c.stradd(k, ": ", c.option.RequestHeader.Get(k), "\r\n")...)
	}
	buf = append(buf, "\r\n"...)
	return buf
}

func (c *dialer) getResponse(br *bufio.Reader) error {
	line, isPrefix, err := br.ReadLine()
	if err != nil {
		return err
	}
	if isPrefix {
		return internal.ErrLongLine
	}
	arr := bytes.Split(line, []byte(" "))
	if len(arr) >= 2 {
		code, _ := strconv.Atoi(string(arr[1]))
		c.resp.StatusCode = code
		c.resp.Proto = string(arr[0])
	}
	if len(arr) != 4 || c.resp.StatusCode != 101 {
		return internal.ErrStatusCode
	}
	header, err := textproto.NewReader(br).ReadMIMEHeader()
	if err != nil {
		return err
	}
	c.resp.Header = http.Header(header)
	return nil
}

func (c *dialer) handshake() (*Conn, *http.Response, error) {
	br := bufio.NewReaderSize(c.conn, c.option.ReadBufferSize)
	telegram := c.generateTelegram()
	if err := internal.WriteN(c.conn, telegram, len(telegram)); err != nil {
		return nil, c.resp, err
	}

	var ch = make(chan error)
	go func() { ch <- c.getResponse(br) }()
	if err := <-ch; err != nil {
		return nil, c.resp, err
	}
	if err := c.checkHeaders(); err != nil {
		return nil, c.resp, err
	}
	var compressEnabled = false
	if c.option.CompressEnabled && strings.Contains(c.resp.Header.Get(internal.SecWebSocketExtensions.Key), "permessage-deflate") {
		compressEnabled = true
	}
	if err := c.conn.SetDeadline(time.Time{}); err != nil {
		return nil, c.resp, err
	}
	if err := setNoDelay(c.conn); err != nil {
		return nil, c.resp, err
	}
	return serveWebSocket(false, c.option.getConfig(), new(sliceMap), c.conn, br, c.eventHandler, compressEnabled), c.resp, nil
}

func (c *dialer) checkHeaders() error {
	if !internal.HttpHeaderEqual(c.resp.Header.Get(internal.Connection.Key), internal.Connection.Val) {
		return internal.ErrHandshake
	}
	if !internal.HttpHeaderEqual(c.resp.Header.Get(internal.Upgrade.Key), internal.Upgrade.Val) {
		return internal.ErrHandshake
	}
	var expectedKey = internal.ComputeAcceptKey(c.option.RequestHeader.Get(internal.SecWebSocketKey.Key))
	var actualKey = c.resp.Header.Get(internal.SecWebSocketAccept.Key)
	if actualKey != expectedKey {
		return internal.ErrHandshake
	}
	return nil
}
