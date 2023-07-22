package gws

import (
	"bufio"
	"bytes"
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
