package gws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"net/url"
)

func Dial(addr string, handler Event, option *ClientOption) (*Conn, error) {
	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()

	var dialer = new(dialer)
	URL, err := url.Parse(addr)
	if err != nil {
		return nil, err
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
		var d = &net.Dialer{Timeout: option.DialTimeout}
		conn, dialError = tls.DialWithDialer(d, "tcp", host, option.TlsConfig)
	default:
		return nil, internal.ErrSchema
	}

	if dialError != nil {
		return nil, dialError
	}

	dialer.option = option
	dialer.conn = conn
	dialer.host = host
	dialer.u = URL
	dialer.eventHandler = handler
	return dialer.handshake()
}

type dialer struct {
	option         *ClientOption
	conn           net.Conn
	host           string
	u              *url.URL
	eventHandler   Event
	responseHeader http.Header
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
	var buf []byte
	buf = append(buf, c.stradd("GET ", c.u.RequestURI(), " HTTP/1.1\r\n")...)
	buf = append(buf, c.stradd("Host: ", c.host, "\r\n")...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Sec-WebSocket-Version: 13\r\n"...)
	buf = append(buf, "Sec-WebSocket-Key: 9iA8mc0M97LEg4k+xtRd+g==\r\n"...)
	//buf = append(buf, "Sec-WebSocket-Extensions: permessage-deflate; client_max_window_bits\r\n"...)
	buf = append(buf, "\r\n"...)
	return buf
}

func (c *dialer) handshake() (*Conn, error) {
	brw := bufio.NewReadWriter(
		bufio.NewReaderSize(c.conn, c.option.ReadBufferSize),
		bufio.NewWriterSize(c.conn, c.option.WriteBufferSize),
	)

	telegram := c.generateTelegram()
	if err := internal.WriteN(brw.Writer, telegram, len(telegram)); err != nil {
		return nil, err
	}
	if err := brw.Writer.Flush(); err != nil {
		return nil, err
	}

	var ch = make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), c.option.HandshakeTimeout)
	defer cancel()

	go func() {
		var header = http.Header{}
		var index = 0
		for {
			line, isPrefix, err := brw.Reader.ReadLine()
			if err != nil {
				ch <- err
				return
			}
			if isPrefix {
				ch <- errors.New("line too long")
				return
			}
			if index == 0 {
				arr := bytes.Split(line, []byte(" "))
				if len(arr) != 4 || !bytes.Equal(arr[1], []byte("101")) {
					ch <- errors.New("status code error")
					return
				}
			} else {
				if len(line) == 0 {
					c.responseHeader = header
					ch <- nil
					return
				}
				arr := bytes.SplitN(line, []byte(": "), 2)
				if len(arr) != 2 {
					ch <- errors.New("header format error")
					return
				}
				header.Set(string(arr[0]), string(arr[1]))
			}
			index++
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout")
		case err := <-ch:
			if err != nil {
				return nil, err
			}
			return serveWebSocket(c.option.getConfig(), new(sliceMap), c.conn, brw, c.eventHandler, c.option.CompressEnabled), nil
		}
	}
}
