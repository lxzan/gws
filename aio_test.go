package gws

import (
	"bufio"
	"bytes"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
)

// 创建用于测试的对等连接
func testNewPeer(config *Upgrader) (server, client *Conn) {
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

// 测试异步写入
func TestConn_WriteAsync(t *testing.T) {
	var as = assert.New(t)
	SetGoLimit(16)

	// 关闭压缩
	t.Run("plain text", func(t *testing.T) {
		var handler = new(webSocketMocker)
		var upgrader = NewUpgrader(func(c *Upgrader) {
			c.EventHandler = handler
		})
		server, client := testNewPeer(upgrader)

		var listA []string
		var listB []string
		var count = 1000

		go func() {
			for i := 0; i < count; i++ {
				var n = internal.AlphabetNumeric.Intn(125)
				var message = internal.AlphabetNumeric.Generate(n)
				listA = append(listA, string(message))
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
				listB = append(listB, string(payload))
				wg.Done()
			}
		}()

		wg.Wait()
		as.ElementsMatch(listA, listB)
	})

	// 开启压缩
	t.Run("compressed text", func(t *testing.T) {
		var handler = new(webSocketMocker)
		var upgrader = NewUpgrader(func(c *Upgrader) {
			c.EventHandler = handler
			c.CompressEnabled = true
			c.CompressionThreshold = 1
		})
		server, client := testNewPeer(upgrader)

		var listA []string
		var listB []string
		const count = 1000

		go func() {
			for i := 0; i < count; i++ {
				var n = internal.AlphabetNumeric.Intn(1024)
				var message = internal.AlphabetNumeric.Generate(n)
				listA = append(listA, string(message))
				server.WriteAsync(OpcodeText, message)
			}
		}()

		var wg sync.WaitGroup
		wg.Add(count)

		go func() {
			for {
				var header = frameHeader{}
				length, err := header.Parse(client.rbuf)
				if err != nil {
					return
				}
				var payload = make([]byte, length)
				if _, err := client.rbuf.Read(payload); err != nil {
					return
				}
				if header.GetRSV1() {
					buf, err := client.decompressor.Decompress(bytes.NewBuffer(payload))
					if err != nil {
						return
					}
					payload = buf.Bytes()
				}
				listB = append(listB, string(payload))
				wg.Done()
			}
		}()

		wg.Wait()
		as.ElementsMatch(listA, listB)
	})
}
