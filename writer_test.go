package gws

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func testWrite(c *Conn, fin bool, opcode Opcode, payload []byte) error {
	var useCompress = c.pd.Enabled && opcode.isDataFrame() && len(payload) >= c.pd.Threshold
	if useCompress {
		var buf = bytes.NewBufferString("")
		err := c.deflater.Compress(internal.Bytes(payload), buf, c.cpsWindow.dict)
		if err != nil {
			return internal.NewError(internal.CloseInternalErr, err)
		}
		payload = buf.Bytes()
	}
	if len(payload) > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	var n = len(payload)
	headerLength, maskBytes := header.GenerateHeader(c.isServer, fin, useCompress, opcode, n, c.nextMaskKey())
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
		for range 64 * 1024 {
			buf.WriteString("a")
		}
		var err = client.WriteMessage(OpcodeText, buf.Bytes())
		assert.NoError(t, err)
		wg.Wait()
	})
}

func TestWriteClose(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
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
		var socket = &Conn{closed: 1, config: server.config}
		socket.WriteMessage(OpcodeText, nil)
		socket.WriteAsync(OpcodeText, nil, nil)
	})

	t.Run("", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled: true,
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

		serverHandler.onClose = func(socket *Conn, err error) {
			if v, ok := err.(*CloseError); ok && string(v.Reason) == "goodbye" {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.WriteClose(1000, []byte("goodbye"))
		assert.NoError(t, err)
		err = client.WriteClose(1000, []byte("goodbye"))
		assert.True(t, errors.Is(err, ErrConnClosed))
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var pd = PermessageDeflate{
			Enabled: true,
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

		serverHandler.onClose = func(socket *Conn, err error) {
			if v, ok := err.(*CloseError); ok && len(v.Reason) == 123 {
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		var err = client.WriteClose(1000, internal.AlphabetNumeric.Generate(internal.ThresholdV1-2))
		assert.NoError(t, err)
		wg.Wait()
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
		for i := range count {
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
			_ = b.Broadcast(key.(*Conn), nil)
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
		for i := range count {
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
			if err := b.Broadcast(key.(*Conn), nil); err == nil {
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
		_ = broadcaster.Broadcast(server, nil)
		wg.Wait()
	})
}

func TestBroadcaster_BroadcastCallback(t *testing.T) {
	t.Run("write succeeds", func(t *testing.T) {
		clientHandler := new(webSocketMocker)
		server, client := newPeer(new(webSocketMocker), nil, clientHandler, nil)
		clientHandler.onMessage = func(socket *Conn, message *Message) {
			message.Close()
		}
		t.Cleanup(func() {
			_ = server.NetConn().Close()
			_ = client.NetConn().Close()
		})
		go client.ReadLoop()

		writeCompleted := make(chan error, 1)
		payload := []byte("broadcast payload")
		broadcaster := NewBroadcaster(OpcodeText, payload)
		assert.NoError(t, broadcaster.Broadcast(server, func(err error) {
			writeCompleted <- err
		}))
		broadcaster.Close()

		select {
		case err := <-writeCompleted:
			assert.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("broadcast callback was not invoked")
		}
	})

	t.Run("write fails", func(t *testing.T) {
		server, client := newPeer(new(webSocketMocker), nil, new(webSocketMocker), nil)
		t.Cleanup(func() {
			_ = server.NetConn().Close()
			_ = client.NetConn().Close()
		})
		assert.NoError(t, server.NetConn().Close())
		writeCompleted := make(chan error, 1)
		broadcaster := NewBroadcaster(OpcodeText, []byte("broadcast payload"))

		assert.NoError(t, broadcaster.Broadcast(server, func(err error) {
			writeCompleted <- err
		}))
		broadcaster.Close()
		select {
		case err := <-writeCompleted:
			assert.Error(t, err)
		case <-time.After(time.Second):
			t.Fatal("broadcast callback was not invoked")
		}
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
		panic("test recovery")
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

func TestConn_SplitReaderFillsSegment(t *testing.T) {
	var content = bytes.Repeat([]byte{1}, segmentSize+3)
	var reader = &fixedChunkReader{data: content, size: 7}
	var sizes []int
	var eofs []bool

	err := new(Conn).splitReader(reader, func(index int, eof bool, p []byte) error {
		sizes = append(sizes, len(p))
		eofs = append(eofs, eof)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, []int{segmentSize, 3}, sizes)
	assert.Equal(t, []bool{false, true}, eofs)
}

type fixedChunkReader struct {
	data []byte
	size int
}

func (c *fixedChunkReader) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	if len(p) > c.size {
		p = p[:c.size]
	}
	if len(p) > len(c.data) {
		p = p[:len(c.data)]
	}
	n := copy(p, c.data)
	c.data = c.data[n:]
	return n, nil
}

func TestConn_WriteFile(t *testing.T) {
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

		var err = server.WriteFile(OpcodeBinary, bytes.NewReader(content))
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

		var err = server.WriteFile(OpcodeBinary, bytes.NewReader(content))
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

		for range count {
			var length = 128*1024 + internal.AlphabetNumeric.Intn(10)
			var content = internal.AlphabetNumeric.Generate(length)
			var err = server.WriteFile(OpcodeBinary, bytes.NewReader(content))
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

		var err = client.WriteFile(OpcodeBinary, bytes.NewReader(content))
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

		var err = client.WriteFile(OpcodeBinary, bytes.NewReader(content))
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
		var err = client.WriteFile(OpcodeBinary, bytes.NewReader(content))
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

		var err = client.WriteFile(OpcodeBinary, bytes.NewReader(content))
		assert.Error(t, err)
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		deflater := newBigDeflater(true, PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 12,
			ClientMaxWindowBits: 12,
		})
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error {
			return nil
		}}
		var reader = &readerWrapper{r: new(writerTo), sw: new(slideWindow)}
		err := deflater.Compress(reader, fw, nil)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		deflater := newBigDeflater(true, PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 12,
			ClientMaxWindowBits: 12,
		})
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error {
			return errors.New("2")
		}}
		var reader = &readerWrapper{r: new(writerTo), sw: new(slideWindow)}
		err := deflater.Compress(reader, fw, nil)
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

func TestConn_WriteCloseReservedCode(t *testing.T) {
	var as = assert.New(t)
	var clientHandler = new(webSocketMocker)
	var wg = sync.WaitGroup{}
	wg.Add(1)
	clientHandler.onClose = func(socket *Conn, err error) {
		wg.Done()
	}
	server, client := newPeer(new(webSocketMocker), &ServerOption{}, clientHandler, &ClientOption{})
	go server.ReadLoop()
	go client.ReadLoop()

	for _, code := range []uint16{1005, 1006, 1015} {
		as.True(errors.Is(server.WriteClose(code, nil), ErrReservedCloseCode))
	}
	as.NoError(server.WriteClose(1000, nil))
	wg.Wait()
}

func TestConn_WriteCloseInvalidReason(t *testing.T) {
	t.Run("invalid utf8 reason", func(t *testing.T) {
		server, _ := newPeer(new(webSocketMocker), &ServerOption{CheckUtf8Enabled: true}, new(webSocketMocker), &ClientOption{})
		var err = server.WriteClose(1000, []byte{0xff, 0xfe})
		assert.True(t, errors.Is(err, ErrTextEncoding))
	})

	t.Run("oversized reason", func(t *testing.T) {
		server, _ := newPeer(new(webSocketMocker), &ServerOption{}, new(webSocketMocker), &ClientOption{})
		var err = server.WriteClose(1000, make([]byte, internal.ThresholdV1-1))
		assert.True(t, errors.Is(err, ErrControlFrameTooLarge))
	})
}

func TestConn_WriteControlFrameSizeLimit(t *testing.T) {
	t.Run("ping oversized", func(t *testing.T) {
		server, client := newPeer(new(webSocketMocker), &ServerOption{}, new(webSocketMocker), &ClientOption{})
		go server.ReadLoop()
		go client.ReadLoop()
		var err = server.WritePing(make([]byte, internal.ThresholdV1+1))
		assert.True(t, errors.Is(err, ErrControlFrameTooLarge))
	})

	t.Run("pong max size", func(t *testing.T) {
		var wg = sync.WaitGroup{}
		wg.Add(1)
		var clientHandler = new(webSocketMocker)
		clientHandler.onPong = func(socket *Conn, payload []byte) {
			if len(payload) == internal.ThresholdV1 {
				wg.Done()
			}
		}
		server, client := newPeer(new(webSocketMocker), &ServerOption{}, clientHandler, &ClientOption{})
		go server.ReadLoop()
		go client.ReadLoop()
		var err = server.WritePong(make([]byte, internal.ThresholdV1))
		assert.NoError(t, err)
		wg.Wait()
	})
}

func TestConn_WriteFailureKeepsCompressionDictionary(t *testing.T) {
	var newCompressedPeer = func() *Conn {
		var pd = PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			Threshold:             1,
		}
		server, _ := newPeer(new(webSocketMocker), &ServerOption{PermessageDeflate: pd}, new(webSocketMocker), &ClientOption{PermessageDeflate: pd})
		return server
	}

	t.Run("failed write does not update dictionary", func(t *testing.T) {
		var server = newCompressedPeer()
		assert.NoError(t, server.NetConn().Close())
		var before = len(server.cpsWindow.dict)
		var err = server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(256))
		assert.Error(t, err)
		assert.Equal(t, before, len(server.cpsWindow.dict))
	})

	t.Run("failed broadcast does not update dictionary", func(t *testing.T) {
		var server = newCompressedPeer()
		assert.NoError(t, server.NetConn().Close())
		var wg = sync.WaitGroup{}
		wg.Add(1)
		var writeErr error
		var broadcaster = NewBroadcaster(OpcodeText, internal.AlphabetNumeric.Generate(256))
		assert.NoError(t, broadcaster.Broadcast(server, func(err error) {
			writeErr = err
			wg.Done()
		}))
		wg.Wait()
		broadcaster.Close()
		assert.Error(t, writeErr)
		assert.Equal(t, 0, len(server.cpsWindow.dict))
	})
}

// TestConn_WriteMessageNoAllocations（写路径零分配断言）位于
// writer_norace_test.go（//go:build !race）：-race 下 runtime 会刻意丢弃
// 约 1/4 的 sync.Pool Put，导致分配数断言机制性抖动。

func TestConn_GenFrameBytesMatchesGenFrame(t *testing.T) {
	var newServer = func(option *ServerOption) *Conn {
		server, _ := newPeer(new(webSocketMocker), option, new(webSocketMocker), &ClientOption{})
		return server
	}
	var assertEqual = func(t *testing.T, conn *Conn, opcode Opcode, payload []byte, cfg frameConfig) {
		frame1, err1 := conn.genFrame(opcode, internal.Bytes(payload), cfg)
		frame2, err2 := conn.genFrameBytes(opcode, payload, cfg)
		assert.Equal(t, err1, err2)
		if err1 != nil {
			return
		}
		assert.Equal(t, frame1.Bytes(), frame2.Bytes())
		binaryPool.Put(frame1)
		binaryPool.Put(frame2)
	}
	var cfg = frameConfig{fin: true, checkEncoding: true}
	var payloads = [][]byte{
		{},
		[]byte("hello"),
		internal.AlphabetNumeric.Generate(125),
		internal.AlphabetNumeric.Generate(126),
		internal.AlphabetNumeric.Generate(65535),
		internal.AlphabetNumeric.Generate(65536),
	}

	t.Run("server frames", func(t *testing.T) {
		var conn = newServer(&ServerOption{})
		for _, opcode := range []Opcode{OpcodeText, OpcodeBinary} {
			for _, payload := range payloads {
				assertEqual(t, conn, opcode, payload, cfg)
			}
		}
		for _, opcode := range []Opcode{OpcodePing, OpcodePong, OpcodeCloseConnection} {
			assertEqual(t, conn, opcode, []byte{}, cfg)
			assertEqual(t, conn, opcode, internal.AlphabetNumeric.Generate(125), cfg)
		}
	})

	t.Run("client masked frames", func(t *testing.T) {
		var normalize = func(frame []byte) []byte {
			var out = make([]byte, len(frame))
			copy(out, frame)
			var headerLength = 2
			switch out[1] & 127 {
			case 126:
				headerLength += 2
			case 127:
				headerLength += 8
			}
			var mask [4]byte
			copy(mask[:], out[headerLength:headerLength+4])
			copy(out[headerLength:headerLength+4], make([]byte, 4))
			internal.MaskXOR(out[headerLength+4:], mask[:])
			return out
		}
		_, client := newPeer(new(webSocketMocker), &ServerOption{}, new(webSocketMocker), &ClientOption{})
		for _, n := range []int{0, 100, 300, 70000} {
			var payload = internal.AlphabetNumeric.Generate(n)
			frame1, err1 := client.genFrame(OpcodeBinary, internal.Bytes(payload), cfg)
			frame2, err2 := client.genFrameBytes(OpcodeBinary, payload, cfg)
			assert.NoError(t, err1)
			assert.NoError(t, err2)
			assert.Equal(t, normalize(frame1.Bytes()), normalize(frame2.Bytes()))
			binaryPool.Put(frame1)
			binaryPool.Put(frame2)
		}
	})

	t.Run("compressed frames", func(t *testing.T) {
		var pd = PermessageDeflate{Enabled: true, ServerContextTakeover: true, Threshold: 1}
		var conn = newServer(&ServerOption{PermessageDeflate: pd})
		var payload = internal.AlphabetNumeric.Generate(1024)
		for _, broadcast := range []bool{false, true} {
			assertEqual(t, conn, OpcodeText, payload, frameConfig{
				fin:           true,
				compress:      true,
				broadcast:     broadcast,
				checkEncoding: true,
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		var conn = newServer(&ServerOption{WriteMaxPayloadSize: 16, CheckUtf8Enabled: true})
		assertEqual(t, conn, OpcodeText, []byte{0xff, 0xfe}, cfg)
		assertEqual(t, conn, OpcodeBinary, internal.AlphabetNumeric.Generate(128), cfg)
		assertEqual(t, conn, OpcodePing, internal.AlphabetNumeric.Generate(126), cfg)
		_, err := conn.genFrameBytes(OpcodeText, []byte{0xff, 0xfe}, cfg)
		assert.True(t, errors.Is(err, ErrTextEncoding))
		_, err = conn.genFrameBytes(OpcodeBinary, internal.AlphabetNumeric.Generate(128), cfg)
		assert.True(t, errors.Is(err, ErrMessageTooLarge))
		_, err = conn.genFrameBytes(OpcodePing, internal.AlphabetNumeric.Generate(126), cfg)
		assert.True(t, errors.Is(err, ErrControlFrameTooLarge))
	})
}

// 记录写出的完整帧字节序列, 用于WriteFile线路字节回归校验
// 不内嵌net.TCPConn: 内嵌会继承writev快速路径方法, 导致net.Buffers.WriteTo分发到零值fd
type writeFileRecorder struct {
	buf bytes.Buffer
}

func (c *writeFileRecorder) Read(p []byte) (int, error)       { return 0, io.EOF }
func (c *writeFileRecorder) Write(p []byte) (int, error)      { return c.buf.Write(p) }
func (c *writeFileRecorder) Close() error                     { return nil }
func (c *writeFileRecorder) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *writeFileRecorder) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c *writeFileRecorder) SetDeadline(time.Time) error      { return nil }
func (c *writeFileRecorder) SetReadDeadline(time.Time) error  { return nil }
func (c *writeFileRecorder) SetWriteDeadline(time.Time) error { return nil }

// TestConn_WriteFileFrameGenerationEquivalence 钉住doWriteFile回调闭包所接收的分段形状
// (二进制/延续opcode × fin × 长度档位)下, genFrame与genFrameBytes两条帧生成路径的逐字节等价性,
// 作为genFrame(internal.Bytes(p))替换为genFrameBytes(p)的回归护栏
func TestConn_WriteFileFrameGenerationEquivalence(t *testing.T) {
	var payloads = [][]byte{
		{},
		[]byte("hello"),
		internal.AlphabetNumeric.Generate(125),
		internal.AlphabetNumeric.Generate(126),
		internal.AlphabetNumeric.Generate(65535),
		internal.AlphabetNumeric.Generate(65536),
		internal.AlphabetNumeric.Generate(segmentSize),
	}
	var identity = func(b []byte) []byte { return b }
	var normalize = func(frame []byte) []byte {
		var out = make([]byte, len(frame))
		copy(out, frame)
		var headerLength = 2
		switch out[1] & 127 {
		case 126:
			headerLength += 2
		case 127:
			headerLength += 8
		}
		var mask [4]byte
		copy(mask[:], out[headerLength:headerLength+4])
		copy(out[headerLength:headerLength+4], make([]byte, 4))
		internal.MaskXOR(out[headerLength+4:], mask[:])
		return out
	}
	var assertEqual = func(t *testing.T, conn *Conn, opcode Opcode, fin bool, payload []byte, normalize func([]byte) []byte) {
		var cfg = frameConfig{fin: fin, compress: false, broadcast: false, checkEncoding: false}
		frame1, err1 := conn.genFrame(opcode, internal.Bytes(payload), cfg)
		frame2, err2 := conn.genFrameBytes(opcode, payload, cfg)
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		// 与doWriteFile中RSV1旁路修补相同的操作
		frame1.Bytes()[0] |= uint8(64)
		frame2.Bytes()[0] |= uint8(64)
		assert.Equal(t, normalize(frame1.Bytes()), normalize(frame2.Bytes()))
		binaryPool.Put(frame1)
		binaryPool.Put(frame2)
	}

	t.Run("server frames", func(t *testing.T) {
		server, _ := newPeer(new(webSocketMocker), &ServerOption{}, new(webSocketMocker), &ClientOption{})
		for _, opcode := range []Opcode{OpcodeBinary, OpcodeContinuation} {
			for _, fin := range []bool{false, true} {
				for _, payload := range payloads {
					assertEqual(t, server, opcode, fin, payload, identity)
				}
			}
		}
	})

	t.Run("client masked frames", func(t *testing.T) {
		_, client := newPeer(new(webSocketMocker), &ServerOption{}, new(webSocketMocker), &ClientOption{})
		for _, opcode := range []Opcode{OpcodeBinary, OpcodeContinuation} {
			for _, fin := range []bool{false, true} {
				for _, payload := range payloads {
					assertEqual(t, client, opcode, fin, payload, normalize)
				}
			}
		}
	})
}

// TestConn_WriteFileWireBytes 逐字节钉住WriteFile的线路输出:
// 非压缩档与独立构造的帧序列对比; 压缩档校验帧结构/往返解压/确定性, 并以改动前基线捕获的金标锁定字节序列
func TestConn_WriteFileWireBytes(t *testing.T) {
	t.Run("compress disabled", func(t *testing.T) {
		var upgrader = NewUpgrader(&BuiltinEventHandler{}, nil)
		var recorder = &writeFileRecorder{}
		var conn = &Conn{
			isServer: true,
			conn:     recorder,
			config:   upgrader.option.getConfig(),
		}
		var payload = make([]byte, 2*segmentSize+3)
		for i := range payload {
			payload[i] = byte(i*7 + 3)
		}
		assert.NoError(t, conn.WriteFile(OpcodeBinary, bytes.NewReader(payload)))

		var expected bytes.Buffer
		var header [10]byte
		header[0] = uint8(OpcodeBinary)
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:10], uint64(segmentSize))
		expected.Write(header[:])
		expected.Write(payload[:segmentSize])

		header[0] = uint8(OpcodeContinuation)
		expected.Write(header[:])
		expected.Write(payload[segmentSize : 2*segmentSize])

		expected.Write([]byte{0x80 | uint8(OpcodeContinuation), 3})
		expected.Write(payload[2*segmentSize:])

		assert.Equal(t, expected.Bytes(), recorder.buf.Bytes())
	})

	t.Run("compress enabled", func(t *testing.T) {
		var newConn = func() (*Conn, *writeFileRecorder) {
			var upgrader = NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
				PermessageDeflate: PermessageDeflate{Enabled: true},
			})
			var recorder = &writeFileRecorder{}
			var conn = &Conn{
				isServer: true,
				conn:     recorder,
				pd:       upgrader.option.PermessageDeflate,
				config:   upgrader.option.getConfig(),
			}
			return conn, recorder
		}
		var parseFrames = func(wire []byte) [][]byte {
			var frames [][]byte
			var i int
			for i < len(wire) {
				var lengthCode = wire[i+1] & 127
				var headerLength = 2
				var payloadLength int
				switch lengthCode {
				case 126:
					headerLength += 2
					payloadLength = int(binary.BigEndian.Uint16(wire[i+2 : i+4]))
				case 127:
					headerLength += 8
					payloadLength = int(binary.BigEndian.Uint64(wire[i+2 : i+10]))
				default:
					payloadLength = int(lengthCode)
				}
				frames = append(frames, wire[i:i+headerLength+payloadLength])
				i += headerLength + payloadLength
			}
			return frames
		}

		// 小载荷单帧金标, 期望值取自改动前基线输出
		var small = bytes.Repeat([]byte("0123456789abcdef"), 64)
		var golden = []byte{
			0xc2, 0x1a,
			0x32, 0x30, 0x34, 0x32, 0x36, 0x31, 0x35, 0x33,
			0xb7, 0xb0, 0x4c, 0x4c, 0x4a, 0x4e, 0x49, 0x4d,
			0x1b, 0xe5, 0x8f, 0xf2, 0x47, 0xf9, 0x23, 0x87, 0x0f, 0x00,
		}
		conn, recorder := newConn()
		assert.NoError(t, conn.WriteFile(OpcodeBinary, bytes.NewReader(small)))
		assert.Equal(t, golden, recorder.buf.Bytes())

		// 大载荷多帧: 双独立连接输出逐字节一致(确定性), 帧结构与往返解压正确
		var large = internal.AlphabetNumeric.Generate(200 * 1024)
		conn, recorder = newConn()
		assert.NoError(t, conn.WriteFile(OpcodeBinary, bytes.NewReader(large)))
		var wire1 = append([]byte(nil), recorder.buf.Bytes()...)

		conn, recorder = newConn()
		assert.NoError(t, conn.WriteFile(OpcodeBinary, bytes.NewReader(large)))
		assert.Equal(t, wire1, recorder.buf.Bytes())

		var frames = parseFrames(wire1)
		assert.GreaterOrEqual(t, len(frames), 2)
		var compressed bytes.Buffer
		for i, frame := range frames {
			assert.Equal(t, byte(0), frame[1]&128)
			if i == 0 {
				assert.Equal(t, byte(0x42), frame[0])
			} else if i == len(frames)-1 {
				assert.Equal(t, byte(0x80), frame[0])
			} else {
				assert.Equal(t, byte(0x00), frame[0])
			}
			var lengthCode = frame[1] & 127
			var headerLength = 2
			switch lengthCode {
			case 126:
				headerLength += 2
			case 127:
				headerLength += 8
			}
			compressed.Write(frame[headerLength:])
		}
		var src = append(compressed.Bytes(), flateTail...)
		var decompressed bytes.Buffer
		_, err := decompressed.ReadFrom(flate.NewReader(bytes.NewReader(src)))
		assert.NoError(t, err)
		assert.Equal(t, large, decompressed.Bytes())
	})
}
