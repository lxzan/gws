package gws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewClient 创建WebSocket客户端
func NewClient(handler Event, option *ClientOption) (client *Conn, responseHeader http.Header, e error) {
	var d = &dialer{eventHandler: handler}
	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()

	URL, err := url.Parse(option.Addr)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, internal.ErrSchema
	}

	if dialError != nil {
		return nil, nil, dialError
	}
	if err := conn.SetDeadline(time.Now().Add(option.DialTimeout)); err != nil {
		return nil, nil, err
	}

	d.host = host
	d.option = option
	d.u = URL
	d.conn = conn
	return d.handshake()
}

type dialer struct {
	option       *ClientOption
	conn         net.Conn
	host         string
	u            *url.URL
	eventHandler Event
}

func (c *dialer) stradd(ss ...string) string {
	var b []byte
	for _, item := range ss {
		b = append(b, item...)
	}
	return string(b)
}

// 生成报文
func (c *dialer) generateTelegram(uri string) []byte {
	c.option.RequestHeader.Set("X-Server", "gws")
	{
		var key [16]byte
		binary.BigEndian.PutUint64(key[0:8], internal.AlphabetNumeric.Uint64())
		binary.BigEndian.PutUint64(key[8:16], internal.AlphabetNumeric.Uint64())
		c.option.RequestHeader.Set(internal.SecWebSocketKey.Key, base64.StdEncoding.EncodeToString(key[0:]))
	}
	if c.option.CompressEnabled {
		c.option.RequestHeader.Set(internal.SecWebSocketExtensions.Key, "permessage-deflate")
	}

	var buf []byte
	buf = append(buf, c.stradd("GET ", uri, " HTTP/1.1\r\n")...)
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

func (c *dialer) handshake() (*Conn, http.Header, error) {
	brw := bufio.NewReadWriter(
		bufio.NewReaderSize(c.conn, c.option.ReadBufferSize),
		bufio.NewWriterSize(c.conn, c.option.WriteBufferSize),
	)
	telegram := c.generateTelegram(c.u.RequestURI())
	if err := internal.WriteN(brw.Writer, telegram, len(telegram)); err != nil {
		return nil, nil, err
	}
	if err := brw.Writer.Flush(); err != nil {
		return nil, nil, err
	}

	var header = http.Header{}
	var ch = make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), c.option.DialTimeout)
	defer cancel()

	go func() {
		var index = 0
		for {
			line, isPrefix, err := brw.Reader.ReadLine()
			if err != nil {
				ch <- err
				return
			}
			if isPrefix {
				ch <- internal.ErrLongLine
				return
			}
			if index == 0 {
				arr := bytes.Split(line, []byte(" "))
				if len(arr) != 4 || !bytes.Equal(arr[1], []byte("101")) {
					ch <- internal.ErrStatusCode
					return
				}
			} else {
				if len(line) == 0 {
					ch <- nil
					return
				}
				arr := strings.Split(string(line), ": ")
				if len(arr) != 2 {
					ch <- internal.ErrHandshake
					return
				}
				header.Set(arr[0], arr[1])
			}
			if len(header) >= 128 {
				ch <- internal.ErrLongLine
				return
			}
			index++
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, internal.ErrDialTimeout
		case err := <-ch:
			if err != nil {
				return nil, nil, err
			}
			ws := serveWebSocket(false, c.option.getConfig(), new(sliceMap), c.conn, brw, c.eventHandler, c.option.CompressEnabled)
			if err := internal.Errors(
				func() error { return c.conn.SetDeadline(time.Time{}) },
				func() error { return c.conn.SetReadDeadline(time.Time{}) },
				func() error { return c.conn.SetWriteDeadline(time.Time{}) },
				func() error { return setNoDelay(c.conn) }); err != nil {
				return nil, nil, err
			}
			return ws, header, nil
		}
	}
}
