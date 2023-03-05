package gws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"github.com/lxzan/gws/internal"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func Dial(handler Event, option *ClientOption) (*Conn, http.Header, error) {
	if option == nil {
		option = new(ClientOption)
	}
	option.initialize()

	var dialer = new(dialer)
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
		var d = &net.Dialer{Timeout: option.DialTimeout}
		conn, dialError = tls.DialWithDialer(d, "tcp", host, option.TlsConfig)
	default:
		return nil, nil, internal.ErrSchema
	}

	if dialError != nil {
		return nil, nil, dialError
	}
	if err := conn.SetDeadline(time.Now().Add(option.DialTimeout)); err != nil {
		return nil, nil, err
	}

	dialer.option = option
	dialer.conn = conn
	dialer.host = host
	dialer.eventHandler = handler
	return dialer.handshake()
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
	var buf []byte
	buf = append(buf, c.stradd("GET ", uri, " HTTP/1.1\r\n")...)
	buf = append(buf, c.stradd("Host: ", c.host, "\r\n")...)
	buf = append(buf, "Connection: Upgrade\r\n"...)
	buf = append(buf, "Upgrade: websocket\r\n"...)
	buf = append(buf, "Sec-WebSocket-Version: 13\r\n"...)
	buf = append(buf, "Sec-WebSocket-Key: 9iA8mc0M97LEg4k+xtRd+g==\r\n"...)
	//buf = append(buf, "Sec-WebSocket-Extensions: permessage-deflate; client_max_window_bits\r\n"...)
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
			ws := serveWebSocket(c.option.getConfig(), new(sliceMap), c.conn, brw, c.eventHandler, c.option.CompressEnabled)
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
