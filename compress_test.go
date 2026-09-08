package gws

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/lxzan/gws/internal"

	"github.com/klauspost/compress/flate"
	"github.com/stretchr/testify/assert"
)

func TestSlideWindow(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var sw = new(slideWindow).initialize(nil, 3)
		sw.Write([]byte("abc"))
		assert.Equal(t, string(sw.dict), "abc")

		sw.Write([]byte("def"))
		assert.Equal(t, string(sw.dict), "abcdef")

		sw.Write([]byte("ghi"))
		assert.Equal(t, string(sw.dict), "bcdefghi")
	})

	t.Run("", func(t *testing.T) {
		var sw = new(slideWindow).initialize(nil, 3)
		sw.Write([]byte("abc"))
		assert.Equal(t, string(sw.dict), "abc")

		sw.Write([]byte("defgh123456789"))
		assert.Equal(t, string(sw.dict), "23456789")
	})

	t.Run("", func(t *testing.T) {
		const size = 4 * 1024
		var sw = slideWindow{enabled: true, size: size}
		for range 1000 {
			var n = internal.AlphabetNumeric.Intn(100)
			sw.Write(internal.AlphabetNumeric.Generate(n))
		}
		assert.Equal(t, len(sw.dict), size)
	})

	t.Run("", func(t *testing.T) {
		const size = 4 * 1024
		for range 10 {
			var sw = slideWindow{enabled: true, size: size, dict: make([]byte, internal.AlphabetNumeric.Intn(size))}
			for range 1000 {
				var n = internal.AlphabetNumeric.Intn(100)
				sw.Write(internal.AlphabetNumeric.Generate(n))
			}
			assert.Equal(t, len(sw.dict), size)
		}
	})
}

func TestNegotiation(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var pd = permessageNegotiation("permessage-deflate; client_no_context_takeover; client_max_window_bits=9")
		assert.Equal(t, pd.ClientMaxWindowBits, 9)
		assert.Equal(t, pd.ServerMaxWindowBits, 15)
		assert.True(t, pd.ServerContextTakeover)
		assert.False(t, pd.ClientContextTakeover)
	})

	t.Run("", func(t *testing.T) {
		var pd = permessageNegotiation("permessage-deflate; client_max_window_bits=9; server_max_window_bits=10")
		assert.Equal(t, pd.ClientMaxWindowBits, 9)
		assert.Equal(t, pd.ServerMaxWindowBits, 10)
		assert.True(t, pd.ServerContextTakeover)
		assert.True(t, pd.ClientContextTakeover)
	})

	// 客户端未offer client_max_window_bits时, 解析结果以0标记"未提供"
	t.Run("client_max_window_bits not offered", func(t *testing.T) {
		var pd = permessageNegotiation("permessage-deflate")
		assert.Equal(t, pd.ClientMaxWindowBits, 0)
		assert.Equal(t, pd.ServerMaxWindowBits, 15)
	})

	// 客户端offer了不带值的client_max_window_bits, 按RFC默认值15处理
	t.Run("client_max_window_bits offered without value", func(t *testing.T) {
		var pd = permessageNegotiation("permessage-deflate; client_max_window_bits")
		assert.Equal(t, pd.ClientMaxWindowBits, 15)
		assert.Equal(t, pd.ServerMaxWindowBits, 15)
	})
}

func TestPermessageNegotiation(t *testing.T) {
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
		assert.Equal(t, client.cpsWindow.size, 1024)
		assert.Equal(t, client.dpsWindow.size, 1024)
		assert.Equal(t, client.pd.ServerContextTakeover, true)
		assert.Equal(t, client.pd.ClientContextTakeover, true)
	})

	t.Run("ok 2", func(t *testing.T) {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
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
		assert.Equal(t, client.cpsWindow.size, 0)
		assert.Equal(t, client.dpsWindow.size, 0)
		assert.Equal(t, client.pd.ServerContextTakeover, false)
		assert.Equal(t, client.pd.ClientContextTakeover, false)
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
				ServerContextTakeover: false,
				ClientContextTakeover: false,
			},
		})
		assert.Equal(t, client.cpsWindow.size, 0)
		assert.Equal(t, client.dpsWindow.size, 0)
		assert.Equal(t, client.pd.ServerContextTakeover, false)
		assert.Equal(t, client.pd.ClientContextTakeover, false)
	})

	t.Run("ok 4", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			socket.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(1024))
		}
		var server = NewServer(serverHandler, &ServerOption{PermessageDeflate: PermessageDeflate{
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
		client.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(1024))
	})

	t.Run("ok 5", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			println(message.Data.String())
		}
		var server = NewServer(serverHandler, &ServerOption{PermessageDeflate: PermessageDeflate{
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
				Threshold:             1,
			},
		})
		_ = client.WriteString("he")
		assert.Equal(t, string(client.cpsWindow.dict), "he")
		_ = client.WriteString("llo")
		assert.Equal(t, string(client.cpsWindow.dict), "hello")
		_ = client.Writev(OpcodeText, []byte(", "), []byte("world!"))
		assert.Equal(t, string(client.cpsWindow.dict), "hello, world!")
	})

	t.Run("fail", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		var server = NewServer(serverHandler, &ServerOption{PermessageDeflate: PermessageDeflate{
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
				Threshold:             1,
			},
		})
		err := client.doWrite(OpcodeText, new(writerTo))
		assert.Equal(t, err.Error(), "1")
	})
}

// flateWriter.Write必须遵守io.Writer契约, 返回实际消费的字节数
// flateWriter.Write must honor the io.Writer contract and return the number of bytes consumed
func TestFlateWriter_Write(t *testing.T) {
	t.Run("returns byte count", func(t *testing.T) {
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error { return nil }}
		var p = internal.AlphabetNumeric.Generate(64)
		var n, err = fw.Write(p)
		assert.NoError(t, err)
		assert.Equal(t, len(p), n)
	})

	t.Run("returns byte count when callback fires", func(t *testing.T) {
		var called = false
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error {
			called = true
			return nil
		}}
		var p1 = internal.AlphabetNumeric.Generate(segmentSize)
		var p2 = internal.AlphabetNumeric.Generate(64)
		var n, err = fw.Write(p1)
		assert.NoError(t, err)
		assert.Equal(t, segmentSize, n)
		n, err = fw.Write(p2)
		assert.NoError(t, err)
		assert.Equal(t, len(p2), n)
		assert.True(t, called)
	})

	t.Run("propagates callback error and reports consumed bytes", func(t *testing.T) {
		var fw = &flateWriter{cb: func(index int, eof bool, p []byte) error { return errors.New("1") }}
		var p1 = internal.AlphabetNumeric.Generate(segmentSize)
		var p2 = internal.AlphabetNumeric.Generate(64)
		_, _ = fw.Write(p1)
		var n, err = fw.Write(p2)
		assert.Error(t, err)
		assert.Equal(t, len(p2), n)
	})
}

// TestStripSyncFlushTail 尾剥离边界矩阵: 输出长度 × 尾4字节是否为同步刷新标记,
// 断言行为与抽取前的内联逻辑逐字节一致
func TestStripSyncFlushTail(t *testing.T) {
	var marker = []byte{0x00, 0x00, 0xff, 0xff}
	var cases = []struct {
		name string
		in   []byte
		want []byte
	}{
		{"empty", nil, nil},
		{"len 1", []byte{0x00}, []byte{0x00}},
		{"len 3 marker prefix", []byte{0x00, 0x00, 0xff}, []byte{0x00, 0x00, 0xff}},
		{"len 3 plain", []byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}},
		{"len 4 marker", append([]byte(nil), marker...), []byte{}},
		{"len 4 off by one", []byte{0x00, 0x00, 0xff, 0xfe}, []byte{0x00, 0x00, 0xff, 0xfe}},
		{"len 4 plain", []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}},
		{"len 5 marker tail", append([]byte{0x99}, marker...), []byte{0x99}},
		{"len 5 marker not at tail", []byte{0x00, 0x00, 0xff, 0xff, 0x01}, []byte{0x00, 0x00, 0xff, 0xff, 0x01}},
		{"len 5 non-marker tail", []byte{0x99, 0x00, 0x00, 0xff, 0xfe}, []byte{0x99, 0x00, 0x00, 0xff, 0xfe}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf = bytes.NewBuffer(append([]byte(nil), tc.in...))
			stripSyncFlushTail(buf)
			assert.Equal(t, tc.want, buf.Bytes())
		})
	}
}

// TestNewCpsWriterMatchesLegacyCreation 钉住newCpsWriter与抽取前两处内联创建分支
// (deflater.initialize / newBigDeflater)的输出等价性: 相同参数与输入下压缩字节逐字节一致
func TestNewCpsWriterMatchesLegacyCreation(t *testing.T) {
	var compressWith = func(w *flate.Writer, payload []byte) []byte {
		var buf = bytes.NewBuffer(nil)
		w.ResetDict(buf, nil)
		_, err := w.Write(payload)
		assert.NoError(t, err)
		assert.NoError(t, w.Flush())
		return buf.Bytes()
	}
	var payload = bytes.Repeat([]byte("gws-newCpsWriter-equivalence "), 256)
	for _, isServer := range []bool{false, true} {
		for _, bits := range []int{15, 12, 9} {
			var options = PermessageDeflate{
				Level:               flate.BestSpeed,
				ServerMaxWindowBits: internal.SelectValue(isServer, bits, 15),
				ClientMaxWindowBits: internal.SelectValue(isServer, 15, bits),
			}
			var got = newCpsWriter(isServer, options)
			var want *flate.Writer
			if bits == 15 {
				want, _ = flate.NewWriter(nil, options.Level)
			} else {
				want, _ = flate.NewWriterWindow(nil, internal.BinaryPow(bits))
			}
			assert.Equal(t, compressWith(want, payload), compressWith(got, payload))
		}
	}
}

func TestDeflater_ContextTakeover_RoundTrip(t *testing.T) {
	t.Run("multi message with shared dictionary", func(t *testing.T) {
		var options = PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
			ServerMaxWindowBits:   15,
			ClientMaxWindowBits:   15,
		}
		var compressor = new(deflater).initialize(false, options, defaultReadMaxPayloadSize)
		var decompressor = new(deflater).initialize(true, options, defaultReadMaxPayloadSize)
		var cpsWindow = new(slideWindow).initialize(nil, options.ClientMaxWindowBits)
		var dpsWindow = new(slideWindow).initialize(nil, options.ServerMaxWindowBits)

		var messages = [][]byte{
			[]byte("hello, websocket compression"),
			[]byte("hello, websocket compression again"),
			internal.AlphabetNumeric.Generate(1024),
			bytes.Repeat([]byte("permessage-deflate context takeover "), 2048),
			[]byte("tail message referencing previous window"),
		}

		for _, expected := range messages {
			var src = bytes.NewBuffer(nil)
			assert.NoError(t, compressor.Compress(internal.Bytes(expected), src, cpsWindow.dict))
			_, _ = cpsWindow.Write(expected)

			var dst, err = decompressor.Decompress(src, dpsWindow.dict)
			assert.NoError(t, err)
			assert.Equal(t, expected, dst.Bytes())
			_, _ = dpsWindow.Write(dst.Bytes())
			binaryPool.Put(dst)
		}
	})

	t.Run("decompress oversized then reuse deflater", func(t *testing.T) {
		var options = PermessageDeflate{
			Enabled:             true,
			ServerMaxWindowBits: 15,
			ClientMaxWindowBits: 15,
		}
		var d = new(deflater).initialize(true, options, 64)
		var payload = internal.AlphabetNumeric.Generate(4096)
		var src = bytes.NewBuffer(nil)
		assert.NoError(t, d.Compress(internal.Bytes(payload), src, nil))

		var dst, err = d.Decompress(src, nil)
		assert.Equal(t, internal.CloseMessageTooLarge, err)
		assert.Nil(t, dst)

		var payload2 = internal.AlphabetNumeric.Generate(64)
		var src2 = bytes.NewBuffer(nil)
		assert.NoError(t, d.Compress(internal.Bytes(payload2), src2, nil))
		dst, err = d.Decompress(src2, nil)
		assert.NoError(t, err)
		assert.Equal(t, payload2, dst.Bytes())
	})
}

func TestDeflater_CompressBytes_MatchesCompress(t *testing.T) {
	var options = PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: true,
		ServerMaxWindowBits:   15,
		ClientMaxWindowBits:   15,
	}
	var compressor = new(deflater).initialize(false, options, defaultReadMaxPayloadSize)
	var decompressor = new(deflater).initialize(true, options, defaultReadMaxPayloadSize)
	var cpsWindow = new(slideWindow).initialize(nil, options.ClientMaxWindowBits)
	var dpsWindow = new(slideWindow).initialize(nil, options.ServerMaxWindowBits)

	var payloads = [][]byte{
		[]byte("hello, websocket compression"),
		bytes.Repeat([]byte("permessage-deflate context takeover "), 2048),
		internal.AlphabetNumeric.Generate(4096),
		githubData,
	}

	for _, payload := range payloads {
		var a = bytes.NewBuffer(nil)
		assert.NoError(t, compressor.Compress(internal.Bytes(payload), a, cpsWindow.dict))
		var b = bytes.NewBuffer(nil)
		assert.NoError(t, compressor.CompressBytes(payload, b, cpsWindow.dict))
		assert.Equal(t, a.Bytes(), b.Bytes())

		var src = bytes.NewBuffer(append([]byte(nil), b.Bytes()...))
		var dst, err = decompressor.Decompress(src, dpsWindow.dict)
		assert.NoError(t, err)
		assert.Equal(t, payload, dst.Bytes())
		binaryPool.Put(dst)

		_, _ = cpsWindow.Write(payload)
		_, _ = dpsWindow.Write(payload)
	}
}

func BenchmarkDeflater_Decompress(b *testing.B) {
	var options = PermessageDeflate{
		Enabled:             true,
		Level:               flate.BestSpeed,
		ServerMaxWindowBits: 15,
		ClientMaxWindowBits: 15,
	}
	var d = new(deflater).initialize(true, options, defaultReadMaxPayloadSize)
	var tmp = bytes.NewBuffer(nil)
	_ = d.Compress(internal.Bytes(githubData), tmp, nil)
	var compressed = make([]byte, tmp.Len(), tmp.Len()+len(flateTail))
	copy(compressed, tmp.Bytes())
	var src = bytes.NewBuffer(make([]byte, 0, len(compressed)+len(flateTail)))

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		src.Reset()
		_, _ = src.Write(compressed)
		dst, err := d.Decompress(src, nil)
		if err != nil {
			b.Fatal(err)
		}
		binaryPool.Put(dst)
	}
}

type writerTo struct{}

func (c *writerTo) CheckEncoding(enabled bool, opcode uint8) bool {
	return true
}

func (c *writerTo) Len() int {
	return 10
}

func (c *writerTo) WriteTo(w io.Writer) (n int64, err error) {
	return 0, errors.New("1")
}

func (c *writerTo) Read(p []byte) (n int, err error) {
	return 0, errors.New("1")
}
