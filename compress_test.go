package gws

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"

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
		for i := 0; i < 1000; i++ {
			var n = internal.AlphabetNumeric.Intn(100)
			sw.Write(internal.AlphabetNumeric.Generate(n))
		}
		assert.Equal(t, len(sw.dict), size)
	})

	t.Run("", func(t *testing.T) {
		const size = 4 * 1024
		for i := 0; i < 10; i++ {
			var sw = slideWindow{enabled: true, size: size, dict: make([]byte, internal.AlphabetNumeric.Intn(size))}
			for j := 0; j < 1000; j++ {
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

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
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
		assert.NoError(t, err)
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

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		})
		assert.NoError(t, err)
		_ = client.WriteString("he")
		assert.Equal(t, string(client.cpsWindow.dict), "he")
		_ = client.WriteString("llo")
		assert.Equal(t, string(client.cpsWindow.dict), "hello")
		_ = client.WriteV(OpcodeText, []byte(", "), []byte("world!"))
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

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Threshold:             1,
			},
		})
		assert.NoError(t, err)
		err = client.doWrite(OpcodeText, new(writerTo))
		assert.Equal(t, err.Error(), "1")
	})
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
