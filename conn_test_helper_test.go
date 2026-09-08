package gws

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitServerReady(t testing.TB, addr string) net.Conn {
	t.Helper()
	var deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 10*time.Millisecond)
		if err == nil {
			return conn
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.FailNow(t, "server not ready on "+addr)
	return nil
}

func dialWithRetry(t testing.TB, handler Event, option *ClientOption) *Conn {
	t.Helper()
	var client *Conn
	var err error
	for i := range 4 {
		client, _, err = NewClient(handler, option)
		if err == nil {
			return client
		}
		if i == 3 || !errors.Is(err, syscall.ECONNREFUSED) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err)
	return client
}

func TestServerStartStability(t *testing.T) {
	for range 10 {
		var addr = ":" + nextPort()
		var server = NewServer(new(BuiltinEventHandler), nil)
		go server.Run(addr)
		_ = waitServerReady(t, "localhost"+addr).Close()
		var client = dialWithRetry(t, new(BuiltinEventHandler), &ClientOption{Addr: "ws://localhost" + addr})
		require.NotNil(t, client)
	}
}
