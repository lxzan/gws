package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	klauspost "github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"net"
	"testing"
)

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
			_ = conn.WriteMessage(OpcodeText, testdata)
		}
	})

	b.Run("compress enabled", func(b *testing.B) {
		var upgrader = NewUpgrader(&BuiltinEventHandler{}, &ServerOption{
			CompressEnabled: true,
		})
		var conn = &Conn{
			conn:            &benchConn{},
			compressEnabled: true,
			config:          upgrader.option.getConfig(),
		}
		for i := 0; i < b.N; i++ {
			_ = conn.WriteMessage(OpcodeText, testdata)
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
		var buf, _, _ = conn1.genFrame(OpcodeText, testdata)

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
		var conn1 = &Conn{
			isServer:        false,
			conn:            &benchConn{},
			compressEnabled: true,
			config:          upgrader.option.getConfig(),
		}
		var buf, _, _ = conn1.genFrame(OpcodeText, testdata)

		var reader = bytes.NewBuffer(buf.Bytes())
		var conn2 = &Conn{
			isServer:        true,
			conn:            &benchConn{},
			rbuf:            bufio.NewReader(reader),
			config:          upgrader.option.getConfig(),
			compressEnabled: true,
			handler:         upgrader.eventHandler,
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
	contents := testdata
	buffer := bytes.NewBuffer(make([]byte, len(testdata)))
	for i := 0; i < b.N; i++ {
		buffer.Reset()
		fw.Reset(buffer)
		fw.Write(contents)
		fw.Flush()
	}
}

func BenchmarkKlauspostCompress(b *testing.B) {
	fw, _ := klauspost.NewWriter(nil, flate.BestSpeed)
	contents := testdata
	buffer := bytes.NewBuffer(make([]byte, len(testdata)))
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
