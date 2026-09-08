package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	_ "embed"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	klauspost "github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

//go:embed assets/github.json
var githubData []byte

type benchConn struct {
	net.TCPConn
}

func (m benchConn) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// 与benchConn同口径的空写连接, 但不内嵌net.TCPConn: 内嵌TCPConn会继承writev快速路径方法,
// net.Buffers.WriteTo将分发到零值fd并返回EINVAL, 导致WriteFile非压缩基准测量错误路径
type benchFileConn struct{}

func (benchFileConn) Read(p []byte) (int, error)       { return 0, io.EOF }
func (benchFileConn) Write(p []byte) (int, error)      { return len(p), nil }
func (benchFileConn) Close() error                     { return nil }
func (benchFileConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (benchFileConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (benchFileConn) SetDeadline(time.Time) error      { return nil }
func (benchFileConn) SetReadDeadline(time.Time) error  { return nil }
func (benchFileConn) SetWriteDeadline(time.Time) error { return nil }

type lazyValue[T any] struct {
	once sync.Once
	v    T
}

func (m *lazyValue[T]) get(create func() T) T {
	m.once.Do(func() { m.v = create() })
	return m.v
}

var (
	benchWriteMessagePlain   lazyValue[*Upgrader]
	benchWriteMessageDeflate lazyValue[*Upgrader]

	benchReadMessagePlain    lazyValue[*Upgrader]
	benchReadMessageDeflate  lazyValue[*Upgrader]
	benchReadMessageDeflater lazyValue[*deflater]

	benchWriteFramePlain   lazyValue[*Upgrader]
	benchWriteFrameDeflate lazyValue[*Upgrader]

	benchWritevPlain   lazyValue[*Upgrader]
	benchWritevDeflate lazyValue[*Upgrader]
	benchWritevPayload lazyValue[[][]byte]

	benchWriteFilePlain   lazyValue[*Upgrader]
	benchWriteFileDeflate lazyValue[*Upgrader]
	benchWriteFileData    lazyValue[[]byte]

	benchNextReaderPlain    lazyValue[*Upgrader]
	benchNextReaderDeflate  lazyValue[*Upgrader]
	benchNextReaderDeflater lazyValue[*deflater]
)

func BenchmarkConn_WriteMessage(b *testing.B) {
	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = benchWriteMessagePlain.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, nil)
		})
		var conn = &Conn{
			conn:   &benchConn{},
			config: upgrader.option.getConfig(),
		}
		b.ResetTimer()
		for range b.N {
			_ = conn.WriteMessage(OpcodeText, githubData)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = benchWriteMessageDeflate.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
				PermessageDeflate: PermessageDeflate{
					Enabled:  true,
					PoolSize: 64,
				},
			})
		})
		var config = upgrader.option.getConfig()
		var conn = &Conn{
			conn:     &benchConn{},
			pd:       PermessageDeflate{Enabled: true, Threshold: defaultCompressThreshold},
			config:   config,
			deflater: upgrader.deflaterPool.Select(),
		}
		_ = conn.WriteMessage(OpcodeText, githubData)
		b.ResetTimer()
		for range b.N {
			_ = conn.WriteMessage(OpcodeText, githubData)
		}
	})
}

func BenchmarkConn_ReadMessage(b *testing.B) {
	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = benchReadMessagePlain.get(func() *Upgrader {
			var handler = &webSocketMocker{}
			handler.onMessage = func(socket *Conn, message *Message) { _ = message.Close() }
			return NewUpgrader(handler, nil)
		})
		var conn1 = &Conn{
			isServer: false,
			conn:     &benchConn{},
			config:   upgrader.option.getConfig(),
		}
		var buf, _ = conn1.genFrame(OpcodeText, internal.Bytes(githubData), frameConfig{
			fin:           true,
			compress:      conn1.pd.Enabled,
			broadcast:     false,
			checkEncoding: false,
		})

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer: true,
			conn:     &benchConn{},
			br:       bufio.NewReader(reader),
			config:   upgrader.option.getConfig(),
			handler:  upgrader.eventHandler,
		}
		b.ResetTimer()
		for range b.N {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_ = conn2.readMessage()
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = benchReadMessageDeflate.get(func() *Upgrader {
			var handler = &webSocketMocker{}
			handler.onMessage = func(socket *Conn, message *Message) { _ = message.Close() }
			return NewUpgrader(handler, &ServerOption{
				PermessageDeflate: PermessageDeflate{Enabled: true},
			})
		})
		var config = upgrader.option.getConfig()
		var cps = benchReadMessageDeflater.get(func() *deflater {
			return new(deflater).initialize(false, upgrader.option.PermessageDeflate, config.ReadMaxPayloadSize)
		})
		var conn1 = &Conn{
			isServer: false,
			conn:     &benchConn{},
			pd:       upgrader.option.PermessageDeflate,
			config:   config,
			deflater: cps,
		}
		var buf, _ = conn1.genFrame(OpcodeText, internal.Bytes(githubData), frameConfig{
			fin:           true,
			compress:      conn1.pd.Enabled,
			broadcast:     false,
			checkEncoding: false,
		})

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer: true,
			conn:     &benchConn{},
			br:       bufio.NewReader(reader),
			config:   upgrader.option.getConfig(),
			pd:       upgrader.option.PermessageDeflate,
			handler:  upgrader.eventHandler,
			deflater: upgrader.deflaterPool.Select(),
		}
		b.ResetTimer()
		for range b.N {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_ = conn2.readMessage()
		}
	})
}

func BenchmarkConn_Writev(b *testing.B) {
	var payloads = benchWritevPayload.get(func() [][]byte {
		var tail = []byte("中文字符🚀")
		var unit = append(append([]byte{}, githubData...), tail...)
		var data []byte
		for range 4 {
			data = append(data, unit...)
		}
		// 切割点取多字节字符的第2个字节, 切片边界截断符文以覆盖跨切片增量UTF-8校验
		var cuts = [3]int{
			len(githubData) + 1,
			len(unit) + len(githubData) + 1,
			2*len(unit) + len(githubData) + 1,
		}
		var payloads = make([][]byte, 0, len(cuts)+1)
		var start int
		for _, cut := range cuts {
			payloads = append(payloads, data[start:cut])
			start = cut
		}
		return append(payloads, data[start:])
	})

	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = benchWritevPlain.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
				CheckUtf8Enabled: true,
			})
		})
		var conn = &Conn{
			conn:   &benchConn{},
			config: upgrader.option.getConfig(),
		}
		if err := conn.Writev(OpcodeText, payloads...); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for range b.N {
			_ = conn.Writev(OpcodeText, payloads...)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = benchWritevDeflate.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
				CheckUtf8Enabled: true,
				PermessageDeflate: PermessageDeflate{
					Enabled:  true,
					PoolSize: 64,
				},
			})
		})
		var conn = &Conn{
			conn:     &benchConn{},
			pd:       PermessageDeflate{Enabled: true, Threshold: defaultCompressThreshold},
			config:   upgrader.option.getConfig(),
			deflater: upgrader.deflaterPool.Select(),
		}
		if err := conn.Writev(OpcodeText, payloads...); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for range b.N {
			_ = conn.Writev(OpcodeText, payloads...)
		}
	})
}

func BenchmarkConn_WriteFile(b *testing.B) {
	var data = benchWriteFileData.get(func() []byte {
		var data = make([]byte, 0, 512*1024+len(githubData))
		for len(data) < 512*1024 {
			data = append(data, githubData...)
		}
		return data
	})

	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = benchWriteFilePlain.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, nil)
		})
		var conn = &Conn{
			isServer: true,
			conn:     &benchFileConn{},
			config:   upgrader.option.getConfig(),
		}
		var reader = bytes.NewReader(data)
		if err := conn.WriteFile(OpcodeBinary, reader); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for range b.N {
			_, _ = reader.Seek(0, io.SeekStart)
			_ = conn.WriteFile(OpcodeBinary, reader)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = benchWriteFileDeflate.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
				PermessageDeflate: PermessageDeflate{
					Enabled:  true,
					PoolSize: 64,
				},
			})
		})
		var conn = &Conn{
			isServer: true,
			conn:     &benchFileConn{},
			pd:       upgrader.option.PermessageDeflate,
			config:   upgrader.option.getConfig(),
		}
		var reader = bytes.NewReader(data)
		if err := conn.WriteFile(OpcodeBinary, reader); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for range b.N {
			_, _ = reader.Seek(0, io.SeekStart)
			_ = conn.WriteFile(OpcodeBinary, reader)
		}
	})
}

func benchReadAll(r io.Reader, buf []byte) (int, error) {
	var total int
	for {
		n, err := r.Read(buf)
		total += n
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

func BenchmarkConn_NextReader(b *testing.B) {
	var scratch = make([]byte, 4*1024)

	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = benchNextReaderPlain.get(func() *Upgrader {
			return NewUpgrader(&webSocketMocker{}, nil)
		})
		var conn1 = &Conn{
			isServer: false,
			conn:     &benchConn{},
			config:   upgrader.option.getConfig(),
		}
		var buf, _ = conn1.genFrame(OpcodeText, internal.Bytes(githubData), frameConfig{
			fin:           true,
			compress:      conn1.pd.Enabled,
			broadcast:     false,
			checkEncoding: false,
		})

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer: true,
			conn:     &benchConn{},
			br:       bufio.NewReader(reader),
			config:   upgrader.option.getConfig(),
			handler:  upgrader.eventHandler,
		}
		_, r, err := conn2.NextReader()
		if err != nil {
			b.Fatal(err)
		}
		if n, err := benchReadAll(r, scratch); err != nil || n != len(githubData) {
			b.Fatalf("unexpected read: n=%d, err=%v", n, err)
		}
		b.ResetTimer()
		for range b.N {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_, r, _ := conn2.NextReader()
			_, _ = benchReadAll(r, scratch)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = benchNextReaderDeflate.get(func() *Upgrader {
			return NewUpgrader(&webSocketMocker{}, &ServerOption{
				PermessageDeflate: PermessageDeflate{Enabled: true},
			})
		})
		var config = upgrader.option.getConfig()
		var cps = benchNextReaderDeflater.get(func() *deflater {
			return new(deflater).initialize(false, upgrader.option.PermessageDeflate, config.ReadMaxPayloadSize)
		})
		var conn1 = &Conn{
			isServer: false,
			conn:     &benchConn{},
			pd:       upgrader.option.PermessageDeflate,
			config:   config,
			deflater: cps,
		}
		var buf, _ = conn1.genFrame(OpcodeText, internal.Bytes(githubData), frameConfig{
			fin:           true,
			compress:      conn1.pd.Enabled,
			broadcast:     false,
			checkEncoding: false,
		})

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer: true,
			conn:     &benchConn{},
			br:       bufio.NewReader(reader),
			config:   config,
			pd:       upgrader.option.PermessageDeflate,
			handler:  upgrader.eventHandler,
			deflater: upgrader.deflaterPool.Select(),
		}
		_, r, err := conn2.NextReader()
		if err != nil {
			b.Fatal(err)
		}
		if n, err := benchReadAll(r, scratch); err != nil || n != len(githubData) {
			b.Fatalf("unexpected read: n=%d, err=%v", n, err)
		}
		b.ResetTimer()
		for range b.N {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_, r, _ := conn2.NextReader()
			_, _ = benchReadAll(r, scratch)
		}
	})
}

func BenchmarkBroadcaster_WriteFrame(b *testing.B) {
	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = benchWriteFramePlain.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, nil)
		})
		var conn = &Conn{
			conn:     &benchConn{},
			config:   upgrader.option.getConfig(),
			isServer: true,
		}
		var bc = NewBroadcaster(OpcodeText, githubData)
		var frame, err = conn.genFrameBytes(bc.opcode, bc.payload, frameConfig{
			fin:           true,
			compress:      conn.pd.Enabled,
			broadcast:     true,
			checkEncoding: conn.config.CheckUtf8Enabled,
		})
		if err != nil {
			b.Fatal(err)
		}
		if err := bc.writeFrame(conn, frame); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for range b.N {
			_ = bc.writeFrame(conn, frame)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = benchWriteFrameDeflate.get(func() *Upgrader {
			return NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
				PermessageDeflate: PermessageDeflate{Enabled: true},
			})
		})
		var config = upgrader.option.getConfig()
		var conn = &Conn{
			conn:     &benchConn{},
			isServer: true,
			pd:       upgrader.option.PermessageDeflate,
			config:   config,
			deflater: upgrader.deflaterPool.Select(),
		}
		var bc = NewBroadcaster(OpcodeText, githubData)
		var frame, err = conn.genFrameBytes(bc.opcode, bc.payload, frameConfig{
			fin:           true,
			compress:      conn.pd.Enabled,
			broadcast:     true,
			checkEncoding: conn.config.CheckUtf8Enabled,
		})
		if err != nil {
			b.Fatal(err)
		}
		if err := bc.writeFrame(conn, frame); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for range b.N {
			_ = bc.writeFrame(conn, frame)
		}
	})
}

func BenchmarkStdCompress(b *testing.B) {
	fw, _ := flate.NewWriter(nil, flate.BestSpeed)
	contents := githubData
	buffer := bytes.NewBuffer(make([]byte, len(githubData)))
	b.ResetTimer()
	for range b.N {
		buffer.Reset()
		fw.Reset(buffer)
		fw.Write(contents)
		fw.Flush()
	}
}

func BenchmarkKlauspostCompress(b *testing.B) {
	fw, _ := klauspost.NewWriter(nil, flate.BestSpeed)
	contents := githubData
	buffer := bytes.NewBuffer(make([]byte, len(githubData)))
	b.ResetTimer()
	for range b.N {
		buffer.Reset()
		fw.Reset(buffer)
		fw.Write(contents)
		fw.Flush()
	}
}

func BenchmarkStdDeCompress(b *testing.B) {
	buffer := bytes.NewBuffer(make([]byte, 0, len(githubData)))
	fw, _ := flate.NewWriter(buffer, flate.BestSpeed)
	contents := githubData
	fw.Write(contents)
	fw.Flush()

	p := make([]byte, 4096)
	fr := flate.NewReader(nil)
	src := bytes.NewBuffer(nil)
	b.ResetTimer()
	for range b.N {
		internal.BufferReset(src, buffer.Bytes())
		_, _ = src.Write(flateTail)
		resetter := fr.(flate.Resetter)
		_ = resetter.Reset(src, nil)
		io.CopyBuffer(io.Discard, fr, p)
	}
}

func BenchmarkKlauspostDeCompress(b *testing.B) {
	buffer := bytes.NewBuffer(make([]byte, 0, len(githubData)))
	fw, _ := klauspost.NewWriter(buffer, klauspost.BestSpeed)
	contents := githubData
	fw.Write(contents)
	fw.Flush()

	fr := klauspost.NewReader(nil)
	src := bytes.NewBuffer(nil)
	b.ResetTimer()
	for range b.N {
		internal.BufferReset(src, buffer.Bytes())
		_, _ = src.Write(flateTail)
		resetter := fr.(klauspost.Resetter)
		_ = resetter.Reset(src, nil)
		fr.(io.WriterTo).WriteTo(io.Discard)
	}
}

func BenchmarkMask(b *testing.B) {
	var s1 = internal.AlphabetNumeric.Generate(1280)
	var s2 = s1
	var key [4]byte
	binary.LittleEndian.PutUint32(key[:4], internal.AlphabetNumeric.Uint32())
	b.ResetTimer()
	for range b.N {
		internal.MaskXOR(s2, key[:4])
	}
}
