package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	_ "embed"
	"encoding/binary"
	klauspost "github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"net"
	"testing"
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
	b.Run("compress disabled", func(b *testing.B) {
		var upgrader = NewUpgrader(&BuiltinEventHandler{}, nil)
		var conn1 = &Conn{
			isServer: false,
			conn:     &benchConn{},
			config:   upgrader.option.getConfig(),
		}
		var buf, _, _ = conn1.genFrame(OpcodeText, githubData)

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer: true,
			conn:     &benchConn{},
			rbuf:     bufio.NewReader(reader),
			config:   upgrader.option.getConfig(),
			handler:  upgrader.eventHandler,
		}
		for i := 0; i < b.N; i++ {
			reader = bytes.NewBuffer(buf.Bytes())
			conn2.rbuf.Reset(reader)
			_ = conn2.readMessage()
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = NewUpgrader(&BuiltinEventHandler{}, &ServerOption{CompressEnabled: true})
		var config = upgrader.option.getConfig()
		var conn1 = &Conn{
			isServer:        false,
			conn:            &benchConn{},
			compressEnabled: true,
			config:          config,
			compressor:      config.compressors.Select(),
			decompressor:    config.decompressors.Select(),
		}
		var buf, _, _ = conn1.genFrame(OpcodeText, githubData)

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer:        true,
			conn:            &benchConn{},
			rbuf:            bufio.NewReader(reader),
			config:          upgrader.option.getConfig(),
			compressEnabled: true,
			handler:         upgrader.eventHandler,
			compressor:      config.compressors.Select(),
			decompressor:    config.decompressors.Select(),
		}
		for i := 0; i < b.N; i++ {
			reader = bytes.NewBuffer(buf.Bytes())
			conn2.rbuf.Reset(reader)
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

func BenchmarkMask(b *testing.B) {
	var s1 = internal.AlphabetNumeric.Generate(1280)
	var s2 = s1
	var key [4]byte
	binary.LittleEndian.PutUint32(key[:4], internal.AlphabetNumeric.Uint32())
	for i := 0; i < b.N; i++ {
		internal.MaskXOR(s2, key[:4])
	}
}
