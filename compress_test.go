package gws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlideWindow(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var sw = new(slideWindow).initialize(3)
		sw.Write([]byte("abc"))
		assert.Equal(t, string(sw.dict), "abc")

		sw.Write([]byte("def"))
		assert.Equal(t, string(sw.dict), "abcdef")

		sw.Write([]byte("ghi"))
		assert.Equal(t, string(sw.dict), "bcdefghi")
	})

	t.Run("", func(t *testing.T) {
		var sw = new(slideWindow).initialize(3)
		sw.Write([]byte("abc"))
		assert.Equal(t, string(sw.dict), "abc")

		sw.Write([]byte("defgh123456789"))
		assert.Equal(t, string(sw.dict), "23456789")
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
}
