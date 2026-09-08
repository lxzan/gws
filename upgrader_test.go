package gws

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nextPort() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	return fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
}

func newHttpWriter() *httpWriter {
	server, client := net.Pipe()
	var r = bytes.NewBuffer(nil)
	var w = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	go func() {
		for {
			var p [1024]byte
			if _, err := client.Read(p[0:]); err != nil {
				return
			}
		}
	}()

	return &httpWriter{
		conn: server,
		brw:  brw,
	}
}

type httpWriter struct {
	conn net.Conn
	brw  *bufio.ReadWriter
}

func (c *httpWriter) Header() http.Header {
	return http.Header{}
}

func (c *httpWriter) Write(i []byte) (int, error) {
	return 0, nil
}

func (c *httpWriter) WriteHeader(statusCode int) {}

func (c *httpWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return c.conn, c.brw, nil
}

type httpWriterWrapper1 struct {
	*httpWriter
}

func (c *httpWriterWrapper1) Hijack() {}

type httpWriterWrapper2 struct {
	*httpWriter
}

func (c *httpWriterWrapper2) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return c.conn, nil, errors.New("test")
}

// 捕获写入内容的网络连接, 用于读取握手响应原文
// Network connection that captures written bytes for inspecting handshake responses
type captureConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *captureConn) Read(p []byte) (int, error)         { return 0, io.EOF }
func (c *captureConn) Close() error                       { return nil }
func (c *captureConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *captureConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (c *captureConn) SetDeadline(t time.Time) error      { return nil }
func (c *captureConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *captureConn) SetWriteDeadline(t time.Time) error { return nil }

func (c *captureConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *captureConn) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// 从握手响应原文中提取Sec-WebSocket-Extensions里指定参数的数值
// Extracts the numeric value of a parameter from the Sec-WebSocket-Extensions response header
func extensionWindowBits(resp, param string) (int, bool) {
	var i = strings.Index(resp, internal.SecWebSocketExtensions.Key+": ")
	if i < 0 {
		return 0, false
	}
	var j = strings.Index(resp[i:], "\r\n")
	if j < 0 {
		return 0, false
	}
	var line = resp[i : i+j]
	var k = strings.Index(line, param)
	if k < 0 {
		return 0, false
	}
	var rest = line[k+len(param):]
	if !strings.HasPrefix(rest, "=") {
		return 0, true
	}
	var end = strings.Index(rest, ";")
	if end < 0 {
		end = len(rest)
	}
	var v, err = strconv.Atoi(rest[1:end])
	if err != nil {
		return 0, false
	}
	return v, true
}

func TestNoDelay(t *testing.T) {
	t.Run("tcp conn", func(t *testing.T) {
		conn := &Conn{conn: &net.TCPConn{}}
		conn.SetNoDelay(false)
	})

	t.Run("tls conn", func(t *testing.T) {
		tlsConn := tls.Client(&net.TCPConn{}, nil)
		conn := &Conn{conn: tlsConn}
		conn.SetNoDelay(false)
	})

	t.Run("other", func(t *testing.T) {
		conn, _ := net.Pipe()
		socket := &Conn{conn: conn}
		socket.SetNoDelay(false)
	})
}

func TestAccept(t *testing.T) {
	var upgrader = NewUpgrader(new(webSocketMocker), &ServerOption{
		PermessageDeflate: PermessageDeflate{Enabled: true},
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		ResponseHeader: http.Header{
			"Server": []string{"gws"},
		},
	})

	t.Run("ok", func(t *testing.T) {
		upgrader.option.PermessageDeflate.Enabled = true
		upgrader.option.SubProtocols = []string{"chat"}
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
		request.Header.Set("Sec-WebSocket-Protocol", "chat")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.NoError(t, err)
	})

	t.Run("fail Sec-WebSocket-Version", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "14")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "client_max_window_bits")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail method", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodPost,
		}
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail Connection", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "up")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail Connection", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "ws")
		request.Header.Set("Sec-WebSocket-Version", "13")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail Sec-WebSocket-Key", func(t *testing.T) {
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})

	t.Run("fail check origin", func(t *testing.T) {
		upgrader.option.PermessageDeflate.Enabled = true
		upgrader.option.Authorize = func(r *http.Request, session SessionStorage) bool {
			return false
		}
		var request = &http.Request{
			Header: http.Header{},
			Method: http.MethodGet,
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
		_, err := upgrader.Upgrade(newHttpWriter(), request)
		assert.Error(t, err)
	})
}

func TestFailHijack(t *testing.T) {
	var upgrader = NewUpgrader(new(webSocketMocker), &ServerOption{
		ResponseHeader: http.Header{"Server": []string{"gws"}},
	})
	var request = &http.Request{
		Header: http.Header{},
		Method: http.MethodGet,
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
	request.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
	_, err := upgrader.Upgrade(&httpWriterWrapper1{httpWriter: newHttpWriter()}, request)
	assert.Error(t, err)

	_, err = upgrader.Upgrade(&httpWriterWrapper2{httpWriter: newHttpWriter()}, request)
	assert.Error(t, err)
}

// RFC7692 §7.1.2.2: 服务端协商的window_bits不得超过客户端offer值;
// 客户端未offer client_max_window_bits时, 服务端响应不得包含该参数.
// The negotiated window bits must not exceed the client's offer;
// client_max_window_bits must not appear in the response if the client didn't offer it.
func TestPermessageDeflate_ServerNegotiation_WindowBits(t *testing.T) {
	var as = assert.New(t)

	var upgrader = NewUpgrader(new(webSocketMocker), &ServerOption{
		PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   13,
			ClientMaxWindowBits:   13,
		},
	})

	upgrade := func(extensions string) string {
		var captured = &captureConn{}
		var w = &httpWriter{
			conn: captured,
			brw:  bufio.NewReadWriter(bufio.NewReader(bytes.NewBuffer(nil)), bufio.NewWriter(io.Discard)),
		}
		var request = &http.Request{Header: http.Header{}, Method: http.MethodGet}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Key", "3tTS/Y+YGaM7TTnPuafHng==")
		request.Header.Set("Sec-WebSocket-Extensions", extensions)
		_, err := upgrader.Upgrade(w, request)
		as.NoError(err)
		return captured.text()
	}

	// 客户端offer了带值的window_bits, 服务端响应不得超过offer值
	t.Run("respects client offered window bits", func(t *testing.T) {
		var resp = upgrade("permessage-deflate; server_max_window_bits=10; client_max_window_bits=11")
		serverBits, ok := extensionWindowBits(resp, "server_max_window_bits")
		as.True(ok, "server_max_window_bits must be present in response")
		as.LessOrEqual(serverBits, 10)
		as.GreaterOrEqual(serverBits, 8)
		clientBits, ok := extensionWindowBits(resp, "client_max_window_bits")
		as.True(ok, "client_max_window_bits must be present in response when offered")
		as.LessOrEqual(clientBits, 11)
		as.GreaterOrEqual(clientBits, 8)
	})

	// 客户端未offer client_max_window_bits, 服务端响应不得包含该参数
	t.Run("omits client_max_window_bits when not offered", func(t *testing.T) {
		var resp = upgrade("permessage-deflate; server_no_context_takeover")
		_, ok := extensionWindowBits(resp, "client_max_window_bits")
		as.False(ok, "client_max_window_bits must not be present when the client didn't offer it")
	})

	// 客户端offer了不带值的client_max_window_bits, 服务端可回复自己的选择
	t.Run("bare client_max_window_bits offer", func(t *testing.T) {
		var resp = upgrade("permessage-deflate; client_max_window_bits")
		clientBits, ok := extensionWindowBits(resp, "client_max_window_bits")
		as.True(ok)
		as.LessOrEqual(clientBits, 13)
		as.GreaterOrEqual(clientBits, 8)
	})
}

func TestNewServer(t *testing.T) {
	var as = assert.New(t)

	t.Run("ok 1", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   10,
			ClientMaxWindowBits:   10,
		}})
		go server.Run(addr)
		_ = waitServerReady(t, "localhost"+addr).Close()

		client := dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		})
		client.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(300*1024))
		client.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(300*1024))
	})

	t.Run("ok 2", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   10,
			ClientMaxWindowBits:   10,
		}})
		go server.Run(addr)
		_ = waitServerReady(t, "localhost"+addr).Close()

		client := dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				ClientMaxWindowBits:   10,
			},
		})
		require.NotNil(t, client)
	})

	t.Run("ok 3", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   10,
			ClientMaxWindowBits:   10,
		}})
		go server.Run(addr)
		_ = waitServerReady(t, "localhost"+addr).Close()

		client := dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				ServerMaxWindowBits:   10,
				ClientMaxWindowBits:   10,
			},
		})
		client.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(300*1024))
		client.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(300*1024))
	})

	t.Run("tls", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		}})
		var dir = os.Getenv("PWD")
		go server.RunTLS(addr, dir+"/examples/wss/cert/server.crt", dir+"/examples/wss/cert/server.pem")
		_ = waitServerReady(t, "localhost"+addr).Close()
	})

	t.Run("fail 1", func(t *testing.T) {
		var addr = ":" + nextPort()
		var wg = sync.WaitGroup{}
		wg.Add(1)
		var server = NewServer(new(BuiltinEventHandler), nil)
		server.OnError = func(conn net.Conn, err error) {
			wg.Done()
		}
		go server.Run(addr)

		client := waitServerReady(t, "localhost"+addr)
		var payload = fmt.Sprintf("POST ws://localhost%s HTTP/1.1\r\n\r\n", addr)
		client.Write([]byte(payload))
		wg.Wait()
	})

	t.Run("fail 2", func(t *testing.T) {
		var addr = ":" + nextPort()
		var wg = sync.WaitGroup{}
		wg.Add(1)
		var server = NewServer(new(BuiltinEventHandler), nil)
		server.OnError = func(conn net.Conn, err error) {
			wg.Done()
		}
		go server.Run(addr)

		client := waitServerReady(t, "localhost"+addr)
		var payload = fmt.Sprintf("GET ws://localhost%s HTTP/1.1 GWS\r\n\r\n", addr)
		client.Write([]byte(payload))
		wg.Wait()
	})

	t.Run("fail 3", func(t *testing.T) {
		var server = NewServer(new(BuiltinEventHandler), nil)
		var addr = ":" + nextPort()
		go server.Run(addr)
		_ = waitServerReady(t, "localhost"+addr).Close()

		as.Error(NewServer(new(BuiltinEventHandler), nil).Run(addr))
		as.Error(NewServer(new(BuiltinEventHandler), nil).RunTLS(addr, "", ""))
		{
			server := NewServer(new(BuiltinEventHandler), nil)
			var dir = "./"
			var tlsAddr = ":" + nextPort()
			go server.RunTLS(tlsAddr, dir+"/examples/wss/cert/server.crt", dir+"/examples/wss/cert/server.pem")
			_ = waitServerReady(t, "localhost"+tlsAddr).Close()
			as.Error(server.RunTLS(addr, dir+"/examples/wss/cert/server.crt", dir+"/examples/wss/cert/server.pem"))
		}
	})
}

func TestBuiltinEventEngine(t *testing.T) {
	{
		var ev = &webSocketMocker{}
		_, ok := any(ev).(Event)
		assert.Equal(t, true, ok)

		ev.OnOpen(nil)
		ev.OnClose(nil, nil)
		ev.OnMessage(nil, &Message{})
		ev.OnPing(nil, nil)
		ev.OnPong(nil, nil)
	}

	{
		var ev = &BuiltinEventHandler{}
		ev.OnMessage(nil, &Message{})
		ev.OnPong(nil, nil)
	}
}

func TestSubprotocol(t *testing.T) {
	t.Run("server close", func(t *testing.T) {
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{SubProtocols: []string{"chat"}})
		go func() { app.Run(addr) }()

		_ = waitServerReady(t, addr).Close()
		_, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{Addr: "ws://" + addr})
		assert.Error(t, err)
	})

	t.Run("client close", func(t *testing.T) {
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{})
		go func() { app.Run(addr) }()

		_ = waitServerReady(t, addr).Close()
		rh := http.Header{}
		rh.Set("Sec-WebSocket-Protocol", "chat")
		_, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr:          "ws://" + addr,
			RequestHeader: rh,
		})
		assert.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{SubProtocols: []string{"chat"}})
		go func() { app.Run(addr) }()

		_ = waitServerReady(t, addr).Close()
		rh := http.Header{}
		rh.Set("Sec-WebSocket-Protocol", "chat")
		client := dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{
			Addr:          "ws://" + addr,
			RequestHeader: rh,
		})
		require.NotNil(t, client)
	})
}

func TestResponseWriter_Write(t *testing.T) {
	t.Run("", func(t *testing.T) {
		conn, _ := net.Pipe()
		rw := &responseWriter{b: bytes.NewBuffer(nil)}
		conn.Close()
		err := rw.Write(conn, time.Second)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		conn, _ := net.Pipe()
		rw := &responseWriter{b: bytes.NewBuffer(nil)}
		err := rw.Write(conn, time.Nanosecond)
		assert.Error(t, err)
	})
}

func TestServer_RunListener(t *testing.T) {
	var s = NewServer(&BuiltinEventHandler{}, nil)
	var ln, _ = net.Listen("tcp", ":"+nextPort())
	_ = ln.Close()
	go func() {
		log.Default().SetOutput(bytes.NewBuffer(nil))
		s.RunListener(ln)
	}()
	time.Sleep(time.Microsecond)
}

// R2-02: 握手请求读取阶段必须受HandshakeTimeout约束,
// 空闲连接应在超时时间内被服务端关闭, 处理协程随之回收, 而不是永久常驻.
// R2-02: handshake request reading must be bounded by HandshakeTimeout;
// idle connections must be closed by the server within the timeout
// and their handler goroutines reclaimed instead of lingering forever.
func TestServer_RunListener_HandshakeReadTimeout(t *testing.T) {
	var as = assert.New(t)
	var addr = ":" + nextPort()
	var server = NewServer(new(BuiltinEventHandler), &ServerOption{
		HandshakeTimeout: 200 * time.Millisecond,
	})
	// 错误回调触发即代表处理协程走到错误路径末尾(回调返回后协程立即退出)
	// The error callback marks the end of the handler goroutine's error path
	var closed int32
	server.OnError = func(conn net.Conn, err error) {
		atomic.AddInt32(&closed, 1)
	}
	go server.Run(addr)

	const n = 3
	var probe = waitServerReady(t, "localhost"+addr)
	defer probe.Close()

	var conns []net.Conn
	for range n {
		conn, err := net.Dial("tcp", "localhost"+addr)
		as.NoError(err)
		conns = append(conns, conn)
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	// 空闲连接应在HandshakeTimeout内被服务端主动关闭;
	// 客户端2秒读超时只是宽松上界, 命中它说明服务端没有回收连接.
	// Idle connections must be closed by the server within HandshakeTimeout;
	// the 2-second client-side deadline is only a loose upper bound,
	// hitting it means the server failed to reclaim the connection.
	for _, conn := range conns {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var buf [1]byte
		_, err := conn.Read(buf[:])
		as.Error(err)
		as.False(errors.Is(err, os.ErrDeadlineExceeded), "idle connection not closed within handshake timeout")
	}

	// 全部空闲连接都必须经由超时进入错误路径被回收
	// Every idle connection must have been reclaimed through the timeout error path
	var deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&closed) < n {
		time.Sleep(10 * time.Millisecond)
	}
	as.GreaterOrEqual(atomic.LoadInt32(&closed), int32(n))
}

// R2-02: 握手读取成功后读超时必须被清除, 升级完成后的读循环不受HandshakeTimeout约束.
// R2-02: after the handshake request has been read, the read deadline must be cleared;
// the post-upgrade read loop must not be bounded by HandshakeTimeout.
func TestServer_RunListener_HandshakeDeadlineClearedAfterUpgrade(t *testing.T) {
	var as = assert.New(t)
	var addr = ":" + nextPort()
	var received = make(chan string, 1)
	var serverHandler = new(webSocketMocker)
	serverHandler.onMessage = func(socket *Conn, message *Message) {
		received <- message.Data.String()
	}
	var server = NewServer(serverHandler, &ServerOption{
		HandshakeTimeout: 500 * time.Millisecond,
	})
	go server.Run(addr)

	var client = dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{
		Addr:             "ws://localhost" + addr,
		HandshakeTimeout: time.Second,
	})
	defer client.NetConn().Close()

	// 停留超过HandshakeTimeout后再发送消息; 若读超时未清除, 服务端读取将失败
	// Stay idle past HandshakeTimeout, then send; an uncleared deadline would break the server read
	time.Sleep(700 * time.Millisecond)
	as.NoError(client.WriteMessage(OpcodeText, []byte("hello")))

	select {
	case msg := <-received:
		as.Equal("hello", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("message not received after the handshake timeout window")
	}
}

// R2-04: Upgrader.Upgrade不得丢弃Hijack返回的bufio中握手请求之后预读的字节,
// 否则客户端随握手请求在同一报文内管道化发出的首帧将被静默丢失.
// R2-04: Upgrader.Upgrade must not drop the bytes already buffered beyond the handshake
// request by the hijacked bufio reader, otherwise a first frame pipelined in the same
// segment as the handshake request is silently lost.
func TestUpgrade_PipelinedFirstFrame(t *testing.T) {
	var as = assert.New(t)

	var received = make(chan string, 1)
	var serverHandler = new(webSocketMocker)
	serverHandler.onMessage = func(socket *Conn, message *Message) {
		received <- message.Data.String()
		_ = message.Close()
	}
	var upgrader = NewUpgrader(serverHandler, &ServerOption{})

	var httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		go socket.ReadLoop()
	}))
	defer httpServer.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(httpServer.URL, "http://"))
	as.NoError(err)
	defer conn.Close()

	// 单次Write同时发出握手请求与首个文本帧, 使服务端读取握手时预读到帧字节
	// A single Write carries both the handshake request and the first text frame,
	// so the server's header read pre-buffers the frame bytes
	var req = "GET / HTTP/1.1\r\n" +
		"Host: " + strings.TrimPrefix(httpServer.URL, "http://") + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: 3tTS/Y+YGaM7TTnPuafHng==\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"\r\n"
	// 带掩码的文本帧"hello": FIN=1, opcode=text, MASK=1, len=5, 掩码全零
	// Masked text frame "hello": FIN=1, opcode=text, MASK=1, len=5, zero mask key
	var frame = []byte{0x81, 0x85, 0x00, 0x00, 0x00, 0x00, 'h', 'e', 'l', 'l', 'o'}
	var buf bytes.Buffer
	buf.WriteString(req)
	buf.Write(frame)
	_, err = conn.Write(buf.Bytes())
	as.NoError(err)

	select {
	case msg := <-received:
		as.Equal("hello", msg)
	case <-time.After(3 * time.Second):
		t.Fatal("pipelined first frame not received")
	}
}

// R2-03: 握手请求解析失败时, 服务端必须关闭连接, 否则fd泄漏
// (客户端将观察到连接在发送垃圾数据后仍然开放).
// R2-03: the server must close the connection when the handshake request cannot be parsed,
// otherwise the fd leaks (the client observes the connection still open after sending garbage).
func TestServer_RunListener_ReadError_ClosesConn(t *testing.T) {
	var as = assert.New(t)
	var addr = ":" + nextPort()
	var server = NewServer(new(BuiltinEventHandler), nil)
	var errCh = make(chan error, 2)
	server.OnError = func(conn net.Conn, err error) {
		errCh <- err
	}
	go server.Run(addr)

	var probe = waitServerReady(t, "localhost"+addr)
	defer probe.Close()

	conn, err := net.Dial("tcp", "localhost"+addr)
	as.NoError(err)
	defer conn.Close()

	_, err = conn.Write([]byte("not a valid http request\r\n\r\n"))
	as.NoError(err)

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("read error not reported")
	}

	// 服务端应已主动关闭连接, 客户端读取在宽松上界内得到EOF而非本地超时
	// The server must have closed the connection; the client read must hit EOF, not its own deadline
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var buf [1]byte
	_, err = conn.Read(buf[:])
	as.Error(err)
	as.False(errors.Is(err, os.ErrDeadlineExceeded), "server did not close connection after read error")
}

// countedBrPool 测试专用的确定性bufio.Reader池: 互斥锁保护的空闲列表加创建计数.
// 与基于sync.Pool的实现不同, 其Put不会被竞态检测器丢弃, 空闲列表也不会被GC清空,
// 因此复用断言具有确定性判别力.
// Deterministic bufio.Reader pool for tests: a mutex-guarded free list with a creation counter.
// Unlike the sync.Pool-backed implementation, its Puts are never dropped by the race detector
// and the free list is never cleared by GC, so reuse assertions stay deterministic.
type countedBrPool struct {
	mu      sync.Mutex
	free    []*bufio.Reader
	created int32
}

func (p *countedBrPool) Get() *bufio.Reader {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.free); n > 0 {
		br := p.free[n-1]
		p.free[n-1] = nil
		p.free = p.free[:n-1]
		return br
	}
	p.created++
	return bufio.NewReaderSize(nil, 1024)
}

func (p *countedBrPool) Put(br *bufio.Reader) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.free = append(p.free, br)
}

func (p *countedBrPool) Created() int32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.created
}

// R2-03: 握手请求解析失败时, 从池中取出的bufio.Reader必须归还.
// 用确定性测试池替换默认池: 错误路径若已归还,
// 随后在同一协程内Get必然复用而不触发新的创建.
// R2-03: the bufio.Reader taken from the pool must be returned when the handshake request
// cannot be parsed. The default pool is swapped for a deterministic test pool; if the error
// path returned the reader, a subsequent Get on the same goroutine must reuse it instead of
// creating a new one.
func TestServer_RunListener_ReadError_RecyclesBufio(t *testing.T) {
	var as = assert.New(t)

	var addr = ":" + nextPort()
	var server = NewServer(new(BuiltinEventHandler), nil)

	var testPool = new(countedBrPool)
	server.option.config.brPool = testPool

	var mu sync.Mutex
	var targetAddr string
	var probeResult = make(chan int32, 1)
	server.OnError = func(conn net.Conn, err error) {
		mu.Lock()
		var matched = targetAddr != "" && conn != nil && conn.RemoteAddr().String() == targetAddr
		mu.Unlock()
		if !matched {
			return
		}
		// 回调与错误路径同协程: 若reader已归还, 此处Get必然复用
		// The callback runs on the error-path goroutine: a returned reader must be reused here
		var br = server.option.config.brPool.Get()
		server.option.config.brPool.Put(br)
		probeResult <- testPool.Created()
	}

	go server.Run(addr)
	var probe = waitServerReady(t, "localhost"+addr)
	defer probe.Close()

	// 一条正常WS连接占用一个池中reader, 使创建计数可区分
	// A healthy WS connection keeps one pooled reader checked out, disambiguating the count
	var client = dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{Addr: "ws://localhost" + addr})
	defer client.NetConn().Close()

	conn, err := net.Dial("tcp", "localhost"+addr)
	as.NoError(err)
	defer conn.Close()
	mu.Lock()
	targetAddr = conn.LocalAddr().String()
	mu.Unlock()

	_, err = conn.Write([]byte("not a valid http request\r\n\r\n"))
	as.NoError(err)

	select {
	case n := <-probeResult:
		// probe(1) + WS客户端(2) + 垃圾连接(3); 已归还则Get复用, 计数保持3
		// probe(1) + WS client(2) + garbage conn(3); a returned reader is reused, keeping the count at 3
		as.Equal(int32(3), n)
	case <-time.After(2 * time.Second):
		t.Fatal("read error not reported")
	}
}
