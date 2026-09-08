package gws

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math"
	"net"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

// 测试同步读
func TestReadSync(t *testing.T) {
	var mu = &sync.Mutex{}
	var listA []string
	var listB []string
	const count = 1000
	var wg = &sync.WaitGroup{}
	wg.Add(count)

	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{PermessageDeflate: PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: false,
		ServerMaxWindowBits:   10,
		ClientMaxWindowBits:   10,
	}}
	var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: true,
	}}

	serverHandler.onMessage = func(socket *Conn, message *Message) {
		mu.Lock()
		listB = append(listB, message.Data.String())
		mu.Unlock()
		wg.Done()
	}

	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()

	for range count {
		var n = internal.AlphabetNumeric.Intn(1024)
		var message = internal.AlphabetNumeric.Generate(n)
		listA = append(listA, string(message))
		client.WriteAsync(OpcodeText, message, nil)
	}

	wg.Wait()
	assert.ElementsMatch(t, listA, listB)
}

func TestConn_EmitReadMessageClosesOnError(t *testing.T) {
	as := assert.New(t)
	conn := &Conn{
		config: initServerOption(&ServerOption{CheckUtf8Enabled: true}).getConfig(),
	}
	msg := &Message{Opcode: OpcodeText, Data: bytes.NewBuffer([]byte{0xff})}

	err := conn.emitReadMessage(msg)

	as.Error(err)
	if e, ok := err.(*internal.Error); as.True(ok) {
		as.Equal(internal.CloseUnsupportedData, e.Code)
	}
	as.Nil(msg.Data)
}

//go:embed assets/read_test.json
var testdata []byte

type testRow struct {
	Title    string `json:"title"`
	Fin      bool   `json:"fin"`
	Opcode   uint8  `json:"opcode"`
	Length   int    `json:"length"`
	Payload  string `json:"payload"`
	RSV2     bool   `json:"rsv2"`
	Expected struct {
		Event  string `json:"event"`
		Code   uint16 `json:"code"`
		Reason string `json:"reason"`
	} `json:"expected"`
}

func TestRead(t *testing.T) {
	var as = assert.New(t)

	var items = make([]testRow, 0)
	if err := json.Unmarshal(testdata, &items); err != nil {
		as.NoError(err)
		return
	}

	for _, item := range items {
		println(item.Title)
		var payload []byte
		if item.Payload == "" {
			payload = internal.AlphabetNumeric.Generate(item.Length)
		} else {
			p, err := hex.DecodeString(item.Payload)
			if err != nil {
				as.NoError(err)
				return
			}
			payload = p
		}

		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			ParallelEnabled:     true,
			CheckUtf8Enabled:    false,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 16 * 1024 * 1024,
			PermessageDeflate:   PermessageDeflate{Enabled: true},
		}
		var clientOption = &ClientOption{
			ParallelEnabled:     true,
			PermessageDeflate:   PermessageDeflate{Enabled: true, ServerContextTakeover: true, ClientContextTakeover: true},
			CheckUtf8Enabled:    true,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 1024 * 1024,
		}

		switch item.Expected.Event {
		case "onMessage":
			clientHandler.onMessage = func(socket *Conn, message *Message) {
				as.Equal(string(payload), message.Data.String())
				wg.Done()
			}
		case "onPing":
			clientHandler.onPing = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onPong":
			clientHandler.onPong = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onClose":
			clientHandler.onClose = func(socket *Conn, err error) {
				if v, ok := err.(*CloseError); ok {
					println(v.Error())
				}
				as.Error(err)
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		go server.ReadLoop()

		if item.Fin {
			server.WriteAsync(Opcode(item.Opcode), testCloneBytes(payload), nil)
		} else {
			testWrite(server, false, Opcode(item.Opcode), testCloneBytes(payload))
		}
		wg.Wait()
	}
}

func TestSegments(t *testing.T) {
	var as = assert.New(t)

	t.Run("valid segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(string(s1)+string(s2), message.Data.String())
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("long segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{ReadMaxPayloadSize: 16}
		var clientOption = &ClientOption{}

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("invalid segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeText, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("illegal compression", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{Enabled: true}}

		var s1 = internal.AlphabetNumeric.Generate(1024)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, true, OpcodeText, testCloneBytes(s1))
		}()
		wg.Wait()
	})

	t.Run("decompress error", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{PermessageDeflate: PermessageDeflate{Enabled: true}}
		var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{Enabled: true}}

		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			var payload = bytes.Repeat([]byte{0xFF}, 64)
			_ = writeRawFrameWithRSV(client, OpcodeText, payload, true, 0x40)
		}()
		wg.Wait()
	})
}

// 解压超限(1009)应透传StatusCode而不是包裹为内部错误(1011), 其他解压错误仍包裹为1011
func TestConn_DecompressMessage(t *testing.T) {
	var as = assert.New(t)

	var serverOption = initServerOption(&ServerOption{
		ReadMaxPayloadSize: 64,
		PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})
	var cfg = serverOption.getConfig()
	var conn = &Conn{
		config:   cfg,
		pd:       serverOption.PermessageDeflate,
		deflater: new(deflater).initialize(true, serverOption.PermessageDeflate, cfg.ReadMaxPayloadSize),
	}

	t.Run("too large returns message too large", func(t *testing.T) {
		var payload = internal.AlphabetNumeric.Generate(4096)
		var compressed = bytes.NewBuffer(nil)
		as.NoError(conn.deflater.Compress(internal.Bytes(payload), compressed, nil))

		var msg = &Message{Opcode: OpcodeBinary, Data: compressed, compressed: true}
		var err = conn.decompressMessage(msg)
		as.Equal(internal.CloseMessageTooLarge, err)
		as.Nil(msg.Data)
	})

	t.Run("invalid deflate returns internal error", func(t *testing.T) {
		var msg = &Message{Opcode: OpcodeBinary, Data: bytes.NewBuffer([]byte("invalid deflate")), compressed: true}
		var err = conn.decompressMessage(msg)
		if e, ok := err.(*internal.Error); as.True(ok) {
			as.Equal(internal.CloseInternalErr, e.Code)
		}
		as.Nil(msg.Data)
	})
}

func TestMessage(t *testing.T) {
	var msg = &Message{
		Opcode: OpcodeText,
		Data:   bytes.NewBufferString("1234"),
	}
	_, _ = msg.Read(make([]byte, 2))
	msg.Close()
}

func TestMessage_CloseIdempotent(t *testing.T) {
	var as = assert.New(t)
	var gcPercent = debug.SetGCPercent(-1)
	defer debug.SetGCPercent(gcPercent)
	var procs = runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(procs)

	var msg = &Message{Opcode: OpcodeText, Data: bytes.NewBufferString("1234"), compressed: true}

	as.NoError(msg.Close())
	as.Nil(msg.Data)
	as.Equal(Opcode(0), msg.Opcode)
	as.False(msg.compressed)
	as.NoError(msg.Close())
	as.NoError(msg.Close())

	var maxMatched = 0
	var pooled = 0
	for range 32 {
		var m = &Message{Opcode: OpcodeBinary, Data: bytes.NewBufferString("x"), compressed: true}
		as.NoError(m.Close())
		as.NoError(m.Close())
		var matched = 0
		for range 8 {
			if messagePool.Get() == m {
				matched++
			}
		}
		maxMatched = max(maxMatched, matched)
		if matched == 1 {
			pooled++
		}
	}
	as.LessOrEqual(maxMatched, 1)
	as.Greater(pooled, 0)
}

func TestReadFrame_PooledMessageReuse(t *testing.T) {
	var as = assert.New(t)
	var mu = &sync.Mutex{}
	var received []string
	var wg = &sync.WaitGroup{}
	wg.Add(3)

	var serverHandler = new(webSocketMocker)
	serverHandler.onMessage = func(socket *Conn, message *Message) {
		mu.Lock()
		received = append(received, message.Data.String())
		mu.Unlock()
		_ = message.Close()
		wg.Done()
	}
	var serverOption = &ServerOption{PermessageDeflate: PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: true,
	}}
	var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: true,
	}}

	server, client := newPeer(serverHandler, serverOption, new(webSocketMocker), clientOption)
	go server.ReadLoop()
	go client.ReadLoop()

	var s1 = internal.AlphabetNumeric.Generate(1024)
	var s2 = internal.AlphabetNumeric.Generate(16)
	var s3 = internal.AlphabetNumeric.Generate(32)

	frame, err := client.genFrame(OpcodeText, internal.Bytes(s1), frameConfig{fin: true, compress: true})
	as.NoError(err)
	_, _ = client.conn.Write(frame.Bytes())
	_ = writeRawFrame(client, OpcodeText, testCloneBytes(s2), true, false)
	_ = testWrite(client, false, OpcodeText, testCloneBytes(s3[:16]))
	_ = testWrite(client, true, OpcodeContinuation, testCloneBytes(s3[16:]))

	var done = make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("read timeout: message dropped or stale pooled Message field leaked")
	}
	mu.Lock()
	as.ElementsMatch([]string{string(s1), string(s2), string(s3)}, received)
	mu.Unlock()
}

func TestFrameHeader_Parse(t *testing.T) {
	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		c.Close()
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, false, OpcodeText, 500, 0)
			c.Write(h[:2])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, false, OpcodeText, 1024*1024, 0)
			c.Write(h[:2])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, false, OpcodeText, 1024*1024, 0)
			c.Write(h[:10])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	// RFC6455 §5.2: 64位长度的最高位必须为0, 否则为协议错误(防止转为int后得到负数)
	var as = assert.New(t)
	var is32Bit = strconv.IntSize == 32

	t.Run("rejects 64-bit length with MSB set", func(t *testing.T) {
		var src = []byte{0x82, 0x7F, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		var fh = frameHeader{}
		var n, err = fh.Parse(bytes.NewReader(src))
		as.Equal(internal.CloseProtocolError, err)
		as.Equal(0, n)
	})

	t.Run("math.MaxInt64 length: platform-dependent", func(t *testing.T) {
		var src = []byte{0x82, 0x7F, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
		var fh = frameHeader{}
		var n, err = fh.Parse(bytes.NewReader(src))
		var length int64 = math.MaxInt64
		if is32Bit {
			as.Equal(internal.CloseProtocolError, err)
			as.Equal(0, n)
		} else {
			as.NoError(err)
			as.Equal(int(length), n)
		}
	})

	t.Run("1<<40 length: platform-dependent", func(t *testing.T) {
		var length int64 = 1 << 40
		var fh = frameHeader{}
		var n, err = fh.Parse(bytes.NewReader(newFrameHeader127(uint64(length))))
		if is32Bit {
			as.Equal(internal.CloseProtocolError, err)
			as.Equal(0, n)
		} else {
			as.NoError(err)
			as.Equal(int(length), n)
		}
	})
}

func TestConn_ReadMessage(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			var p = []byte("123")
			frame, _ := socket.genFrame(OpcodePing, internal.Bytes(p), frameConfig{
				fin:           true,
				compress:      socket.pd.Enabled,
				broadcast:     false,
				checkEncoding: socket.config.CheckUtf8Enabled,
			})
			socket.conn.Write(frame.Bytes()[:2])
			socket.conn.Close()
		}
		var server = NewServer(serverHandler, nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		})
		assert.NoError(t, err)
		client.ReadLoop()
	})

	t.Run("", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			var p = []byte("123")
			frame, _ := socket.genFrame(OpcodeText, internal.Bytes(p), frameConfig{
				fin:           true,
				compress:      socket.pd.Enabled,
				broadcast:     false,
				checkEncoding: false,
			})
			socket.conn.Write(frame.Bytes()[:2])
			socket.conn.Close()
		}
		var server = NewServer(serverHandler, nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		})
		assert.NoError(t, err)
		client.ReadLoop()
	})
}
