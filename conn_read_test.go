package gws

import (
	"io"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

// readAll 用小 buf 反复调用 Read, 直到读满 n 个字节
func readAllN(t *testing.T, c *Conn, n int, bufSize int) []byte {
	t.Helper()
	var got = make([]byte, 0, n)
	var buf = make([]byte, bufSize)
	for len(got) < n {
		m, err := c.Read(buf)
		assert.NoError(t, err)
		got = append(got, buf[:m]...)
	}
	return got
}

func TestConn_Read(t *testing.T) {
	var as = assert.New(t)

	t.Run("small buffer stops at message boundary", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		msg1 := internal.AlphabetNumeric.Generate(10)
		msg2 := internal.AlphabetNumeric.Generate(10)

		go func() {
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg1))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg2))
		}()

		got1 := readAllN(t, server, len(msg1), 3)
		as.Equal(string(msg1), string(got1))

		got2 := readAllN(t, server, len(msg2), 3)
		as.Equal(string(msg2), string(got2))
	})

	t.Run("large buffer does not wait for next message", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		msg1 := internal.AlphabetNumeric.Generate(10)

		go func() { _ = testWrite(client, true, OpcodeText, testCloneBytes(msg1)) }()

		var buf = make([]byte, 4096)
		done := make(chan struct{})
		var n int
		var err error
		go func() {
			n, err = server.Read(buf)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Read blocked waiting for a message that was never sent")
		}

		as.NoError(err)
		as.Equal(string(msg1), string(buf[:n]))
	})

	t.Run("fragmented message reassembled transparently", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		want := string(s1) + string(s2)

		go func() {
			_ = testWrite(client, false, OpcodeText, testCloneBytes(s1))
			_ = testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()

		got := readAllN(t, server, len(want), 5)
		as.Equal(want, string(got))
	})

	t.Run("compressed message", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: false,
			ServerMaxWindowBits:   10,
			ClientMaxWindowBits:   10,
		}}
		clientOption := &ClientOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		}}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		msg := internal.AlphabetNumeric.Generate(2048)
		go func() { client.WriteAsync(OpcodeText, testCloneBytes(msg), nil) }()

		got := readAllN(t, server, len(msg), 64)
		as.Equal(string(msg), string(got))
	})

	t.Run("read error propagated like io.Reader", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		_ = client.NetConn().Close()

		var buf = make([]byte, 16)
		_, err := server.Read(buf)
		as.Error(err)
		as.True(err == io.EOF || err != nil)
	})
}
