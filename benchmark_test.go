package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	_ "embed"
	"encoding/binary"
	"io"
	"net"
	"testing"

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

func BenchmarkConn_WriteMessage(b *testing.B) {
	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = NewUpgrader(&BuiltinEventHandler{}, nil)
		var conn = &Conn{
			conn:   &benchConn{},
			config: upgrader.option.getConfig(),
		}
		for i := 0; i < b.N; i++ {
			_ = conn.WriteMessage(OpcodeText, githubData)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
			CompressEnabled: true,
			CompressorNum:   64,
		})
		var config = upgrader.option.getConfig()
		var conn = &Conn{
			conn:            &benchConn{},
			compressEnabled: true,
			config:          config,
			compressor:      config.compressors.Select(),
			decompressor:    config.decompressors.Select(),
		}
		for i := 0; i < b.N; i++ {
			_ = conn.WriteMessage(OpcodeText, githubData)
		}
	})
}

func BenchmarkConn_ReadMessage(b *testing.B) {
	var handler = &webSocketMocker{}
	handler.onMessage = func(socket *Conn, message *Message) { _ = message.Close() }

	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = NewUpgrader(handler, nil)
		var conn1 = &Conn{
			isServer: false,
			conn:     &benchConn{},
			config:   upgrader.option.getConfig(),
		}
		var buf, _ = conn1.genFrame(OpcodeText, githubData)

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer: true,
			conn:     &benchConn{},
			br:       bufio.NewReader(reader),
			config:   upgrader.option.getConfig(),
			handler:  upgrader.eventHandler,
		}
		for i := 0; i < b.N; i++ {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_ = conn2.readMessage()
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = NewUpgrader(handler, &ServerOption{CompressEnabled: true})
		var config = upgrader.option.getConfig()
		var conn1 = &Conn{
			isServer:        false,
			conn:            &benchConn{},
			compressEnabled: true,
			config:          config,
			compressor:      config.compressors.Select(),
			decompressor:    config.decompressors.Select(),
		}
		var buf, _ = conn1.genFrame(OpcodeText, githubData)

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer:        true,
			conn:            &benchConn{},
			br:              bufio.NewReader(reader),
			config:          upgrader.option.getConfig(),
			compressEnabled: true,
			handler:         upgrader.eventHandler,
			compressor:      config.compressors.Select(),
			decompressor:    config.decompressors.Select(),
		}
		for i := 0; i < b.N; i++ {
			internal.BufferReset(reader, buf.Bytes())
			conn2.br.Reset(reader)
			_ = conn2.readMessage()
		}
	})
}

func BenchmarkStdCompress(b *testing.B) {
	fw, _ := flate.NewWriter(nil, flate.BestSpeed)
	contents := githubData
	buffer := bytes.NewBuffer(make([]byte, len(githubData)))
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
		internal.MaskXOR(s2, key[:4])
	}
}
