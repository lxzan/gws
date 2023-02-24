package gws

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
)

func newPeer(config *Upgrader) (server, client *Conn) {
	size := 4096
	s, c := net.Pipe()
	{
		brw := bufio.NewReadWriter(bufio.NewReaderSize(s, size), bufio.NewWriterSize(s, size))
		server = serveWebSocket(config, &Request{}, s, brw, config.EventHandler, config.CompressEnabled)
	}
	{
		brw := bufio.NewReadWriter(bufio.NewReaderSize(c, size), bufio.NewWriterSize(c, size))
		client = serveWebSocket(config, &Request{}, c, brw, config.EventHandler, config.CompressEnabled)
	}
	return
}

func TestConn_WriteAsync(t *testing.T) {
	var as = assert.New(t)
	SetMaxConcurrencyForWriteQueue(8)
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(func(c *Upgrader) {
		c.EventHandler = handler
	})
	server, client := newPeer(upgrader)

	var message = []byte("hello")
	var count = 1000

	go func() {
		for i := 0; i < count; i++ {
			server.WriteAsync(OpcodeText, message)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(count)
	go func() {
		for {
			var header = frameHeader{}
			_, err := client.conn.Read(header[:2])
			if err != nil {
				return
			}
			var payload = make([]byte, header.GetLengthCode())
			if _, err := client.conn.Read(payload); err != nil {
				return
			}
			as.Equal(string(message), string(payload))
			wg.Done()
		}
	}()
	wg.Wait()
}
