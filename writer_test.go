package gws

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func testWrite(c *Conn, fin bool, opcode Opcode, payload []byte) error {
	var useCompress = c.pd.Enabled && opcode.isDataFrame() && len(payload) >= c.pd.Threshold
	if useCompress {
		var buf = bytes.NewBufferString("")
		err := c.deflater.Compress(internal.Bytes(payload), buf, c.cpsWindow.dict)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = buf.Bytes()
	}
	if len(payload) > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	var n = len(payload)
	headerLength, maskBytes := header.GenerateHeader(c.isServer, fin, useCompress, opcode, n)
	if !c.isServer {
		internal.MaskXOR(payload, maskBytes)
	}

	var buf = make(net.Buffers, 0, 2)
	buf = append(buf, header[:headerLength])
	if n > 0 {
		buf = append(buf, payload)
	}
	num, err := buf.WriteTo(c.conn)
	if err != nil {
		return err
	}
	if int(num) < headerLength+n {
		return io.ErrShortWrite
	}
	return nil
}

func TestWriteBigMessage(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{WriteMaxPayloadSize: 16}
		var clientOption = &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		var err = server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			WriteMaxPayloadSize: 16,
			PermessageDeflate:   PermessageDeflate{Enabled: true, Threshold: 1},
		}
		var clientOption = &ClientOption{
			PermessageDeflate: PermessageDeflate{Enabled: true},
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		var err = server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		serverHandler.onClose = func(socket *Conn, err error) {
			assert.True(t, errors.Is(err, internal.CloseMessageTooLarge))
			wg.Done()
		}
		var serverOption = &ServerOption{
			ReadMaxPayloadSize: 128,
			PermessageDeflate:  PermessageDeflate{Enabled: true, Threshold: 1},
		}
		var clientOption = &ClientOption{
			ReadMaxPayloadSize: 128 * 1024,
			PermessageDeflate:  PermessageDeflate{Enabled: true, Threshold: 1},
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var buf = bytes.NewBufferString("")
		for i := 0; i < 64*1024; i++ {
			buf.WriteString("a")
		}
		var err = client.WriteMessage(OpcodeText, buf.Bytes())
		assert.NoError(t, err)
		wg.Wait()
	})
}

func TestWriteClose(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{}
	var clientOption = &ClientOption{}

	var wg = sync.WaitGroup{}
	wg.Add(1)
	serverHandler.onClose = func(socket *Conn, err error) {
		as.Error(err)
		wg.Done()
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	server.WriteClose(1000, []byte("goodbye"))
	wg.Wait()

	t.Run("", func(t *testing.T) {
		var socket = &Conn{closed: 1, config: server.config}
		socket.WriteMessage(OpcodeText, nil)
		socket.WriteAsync(OpcodeText, nil, nil)
	})
}

func TestConn_WriteAsyncError(t *testing.T) {
	t.Run("write async", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		server, _ := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		server.closed = 1
		server.WriteAsync(OpcodeText, nil, nil)
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{CheckUtf8Enabled: true}
		var clientOption = &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		server.WriteAsync(OpcodeText, flateTail, func(err error) {
			assert.Error(t, err)
		})
	})
}

func TestConn_WriteInvalidUTF8(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	var payload = []byte{1, 2, 255}
	as.Error(server.WriteMessage(OpcodeText, payload))
}

func TestConn_WriteClose(t *testing.T) {
	var wg = sync.WaitGroup{}
	wg.Add(3)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	clientHandler.onClose = func(socket *Conn, err error) {
		wg.Done()
	}
	clientHandler.onMessage = func(socket *Conn, message *Message) {
		wg.Done()
	}
	go server.ReadLoop()
	go client.ReadLoop()

	server.WriteMessage(OpcodeText, nil)
	server.WriteMessage(OpcodeText, []byte("hello"))
	server.WriteMessage(OpcodeCloseConnection, []byte{1})
	wg.Wait()
}

func TestNewBroadcaster(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var handler = &broadcastHandler{sockets: &sync.Map{}, wg: &sync.WaitGroup{}}
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{
			PermessageDeflate: PermessageDeflate{Enabled: true},
		})

		app.OnRequest = func(netConn net.Conn, br *bufio.Reader, r *http.Request) {
			socket, err := app.GetUpgrader().UpgradeFromConn(netConn, br, r)
			if err != nil {
				return
			}
			handler.sockets.Store(socket, struct{}{})
			socket.ReadLoop()
		}
		go func() {
			if err := app.Run(addr); err != nil {
				as.NoError(err)
				return
			}
		}()

		time.Sleep(100 * time.Millisecond)

		var count = 100
		for i := 0; i < count; i++ {
			compress := i%2 == 0
			client, _, err := NewClient(handler, &ClientOption{
				Addr:              "ws://" + addr,
				PermessageDeflate: PermessageDeflate{Enabled: compress},
			})
			if err != nil {
				as.NoError(err)
				return
			}
			_ = client.WritePing(nil)
			go client.ReadLoop()
		}

		handler.wg.Add(count)
		var b = NewBroadcaster(OpcodeText, internal.AlphabetNumeric.Generate(1000))
		handler.sockets.Range(func(key, value any) bool {
			_ = b.Broadcast(key.(*Conn))
			return true
		})
		b.Close()
		handler.wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var handler = &broadcastHandler{sockets: &sync.Map{}, wg: &sync.WaitGroup{}}
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{
			PermessageDeflate:   PermessageDeflate{Enabled: true},
			WriteMaxPayloadSize: 1000,
			Authorize: func(r *http.Request, session SessionStorage) bool {
				session.Store("name", 1)
				session.Store("name", 2)
				return true
			},
		})

		app.OnRequest = func(netConn net.Conn, br *bufio.Reader, r *http.Request) {
			socket, err := app.GetUpgrader().UpgradeFromConn(netConn, br, r)
			if err != nil {
				return
			}
			name, _ := socket.Session().Load("name")
			as.Equal(2, name)
			handler.sockets.Store(socket, struct{}{})
			socket.ReadLoop()
		}

		go func() {
			if err := app.Run(addr); err != nil {
				as.NoError(err)
				return
			}
		}()

		time.Sleep(100 * time.Millisecond)

		var count = 100
		for i := 0; i < count; i++ {
			compress := i%2 == 0
			client, _, err := NewClient(handler, &ClientOption{
				Addr:              "ws://" + addr,
				PermessageDeflate: PermessageDeflate{Enabled: compress},
			})
			if err != nil {
				as.NoError(err)
				return
			}
			go client.ReadLoop()
		}

		var b = NewBroadcaster(OpcodeText, testdata)
		handler.sockets.Range(func(key, value any) bool {
			if err := b.Broadcast(key.(*Conn)); err == nil {
				handler.wg.Add(1)
			}
			return true
		})
		time.Sleep(500 * time.Millisecond)
		b.Close()
		handler.wg.Wait()
	})

	t.Run("conn closed", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		server.WriteClose(0, nil)
		var broadcaster = NewBroadcaster(OpcodeText, internal.AlphabetNumeric.Generate(16))
		_ = broadcaster.Broadcast(server)
		wg.Wait()
	})
}

type broadcastHandler struct {
	BuiltinEventHandler
	wg      *sync.WaitGroup
	sockets *sync.Map
}

func (b broadcastHandler) OnMessage(socket *Conn, message *Message) {
	defer message.Close()
	b.wg.Done()
}

func TestRecovery(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{Recovery: Recovery}
	var clientOption = &ClientOption{}
	serverHandler.onMessage = func(socket *Conn, message *Message) {
		var m map[string]uint8
		m[""] = 1
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	as.NoError(client.WriteString("hi"))
	time.Sleep(100 * time.Millisecond)
}

func TestConn_Writev(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), []byte("hello, world!")) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.Writev(OpcodeText, [][]byte{
			[]byte("he"),
			[]byte("llo"),
			[]byte(", world!"),
		}...)
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), []byte("hello, world!")) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		client.WritevAsync(OpcodeText, [][]byte{
			[]byte("he"),
			[]byte("llo"),
			[]byte(", world!"),
		}, func(err error) {
			assert.NoError(t, err)
		})
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}
		var clientOption = &ClientOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), []byte("hello, world!")) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.Writev(OpcodeText, [][]byte{
			[]byte("he"),
			[]byte("llo"),
			[]byte(", world!"),
		}...)
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}
		var clientOption = &ClientOption{
			CheckUtf8Enabled: true,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.Writev(OpcodeText, [][]byte{
			[]byte("山高月小"),
			[]byte("水落石出")[2:],
		}...)
		assert.Error(t, err)
	})
}

func TestConn_Async(t *testing.T) {
	var conn = &Conn{writeQueue: workerQueue{maxConcurrency: 1}}
	var wg = sync.WaitGroup{}
	wg.Add(100)
	var arr1, arr2 []int64
	var mu = &sync.Mutex{}
	for i := 1; i <= 100; i++ {
		var x = int64(i)
		arr1 = append(arr1, x)
		conn.Async(func() {
			mu.Lock()
			arr2 = append(arr2, x)
			mu.Unlock()
			wg.Done()
		})
	}
	wg.Wait()
	assert.True(t, internal.IsSameSlice(arr1, arr2))
}

func TestConn_WriteReader(t *testing.T) {
	t.Run("context_take_over 1", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			Threshold:             1,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate: pd,
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var content = internal.AlphabetNumeric.Generate(512 * 1024)
		clientHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = server.WriteReader(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("context_take_over 2", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   15,
			ClientMaxWindowBits:   15,
			Threshold:             1,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate: pd,
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var content = internal.AlphabetNumeric.Generate(512 * 1024)
		clientHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = server.WriteReader(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("context_take_over 3", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   15,
			ClientMaxWindowBits:   15,
			Threshold:             1,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate: pd,
		}
		var count = 1000
		var wg = &sync.WaitGroup{}
		wg.Add(count)

		clientHandler.onMessage = func(socket *Conn, message *Message) {
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		for i := 0; i < count; i++ {
			var length = 128*1024 + internal.AlphabetNumeric.Intn(10)
			var content = internal.AlphabetNumeric.Generate(length)
			var err = server.WriteReader(OpcodeBinary, bytes.NewReader(content))
			assert.NoError(t, err)
		}
		wg.Wait()
	})

	t.Run("no_context_take_over", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
			Threshold:             1,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate: pd,
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var content = internal.AlphabetNumeric.Generate(512 * 1024)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.WriteReader(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("no_compress", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled: false,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate: pd,
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var content = internal.AlphabetNumeric.Generate(512 * 1024)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if bytes.Equal(message.Bytes(), content) {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.WriteReader(OpcodeBinary, bytes.NewReader(content))
		assert.NoError(t, err)
		wg.Wait()
	})

	t.Run("close 1", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled: false,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate: pd,
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var content = internal.AlphabetNumeric.Generate(512 * 1024)
		serverHandler.onClose = func(socket *Conn, err error) {
			if ev, ok := err.(*CloseError); ok && ev.Code == 1000 {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		client.WriteClose(1000, nil)
		var err = client.WriteReader(OpcodeBinary, bytes.NewReader(content))
		assert.Error(t, err)
		wg.Wait()
	})

	t.Run("msg too big", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled: false,
		}
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			PermessageDeflate: pd,
		}
		var clientOption = &ClientOption{
			PermessageDeflate:   pd,
			WriteMaxPayloadSize: 1024,
		}
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var content = internal.AlphabetNumeric.Generate(512 * 1024)
		clientHandler.onClose = func(socket *Conn, err error) {
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.WriteReader(OpcodeBinary, bytes.NewReader(content))
		assert.Error(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		deflater := new(bigDeflater).initialize(true, PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 12,
			ClientMaxWindowBits: 12,
		})
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error {
			return nil
		}}
		err := deflater.Compress(new(writerTo), fw, nil, new(slideWindow))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		deflater := new(bigDeflater).initialize(true, PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 12,
			ClientMaxWindowBits: 12,
		})
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error {
			return errors.New("2")
		}}
		src := bytes.NewBufferString("hello")
		err := deflater.Compress(src, fw, nil, new(slideWindow))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		var fw = &flateWriter{
			cb: func(index int, eof bool, p []byte) error {
				return nil
			},
			buffers: []*bytes.Buffer{
				bytes.NewBufferString("he"),
				bytes.NewBufferString("llo"),
			},
		}
		var err = fw.Flush()
		assert.NoError(t, err)
	})
}
