package gws

import (
	"bytes"
	"net"
	"sync"
	"testing"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func testWrite(c *Conn, fin bool, opcode Opcode, payload []byte) error {
	var useCompress = c.compressEnabled && opcode.isDataFrame() && len(payload) >= c.config.CompressThreshold
	if useCompress {
		var buf = bytes.NewBufferString("")
		err := c.config.compressors.Select().Compress(payload, buf)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		payload = buf.Bytes()
	}
	if len(payload) > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	var n = len(payload)
	headerLength, maskBytes := header.GenerateHeader(c.isServer, fin, useCompress, opcode, n)
	if !c.isServer {
		internal.MaskXOR(payload, maskBytes)
	}

	var buf = make(net.Buffers, 0, 2)
	buf = append(buf, header[:headerLength])
	if n > 0 {
		buf = append(buf, payload)
	}
	num, err := buf.WriteTo(c.conn)
	return internal.CheckIOError(headerLength+n, int(num), err)
}

func TestWriteBigMessage(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{WriteMaxPayloadSize: 16}
		var clientOption = &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		var err = server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{WriteMaxPayloadSize: 16, CompressEnabled: true, CompressThreshold: 1}
		var clientOption = &ClientOption{CompressEnabled: true}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()
		var err = server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
		assert.Error(t, err)
	})
}

func TestWriteClose(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{}
	var clientOption = &ClientOption{}

	var wg = sync.WaitGroup{}
	wg.Add(1)
	serverHandler.onClose = func(socket *Conn, err error) {
		as.Error(err)
		wg.Done()
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	server.WriteClose(1000, []byte("goodbye"))
	wg.Wait()

	t.Run("", func(t *testing.T) {
		var socket = &Conn{closed: 1, config: server.config}
		socket.WriteMessage(OpcodeText, nil)
		socket.WriteAsync(OpcodeText, nil)
	})
}

func TestConn_WriteAsyncError(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		server, _ := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		server.closed = 1
		server.WriteAsync(OpcodeText, nil)
	})
}

func TestConn_WriteInvalidUTF8(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()
	var payload = []byte{1, 2, 255}
	as.Error(server.WriteMessage(OpcodeText, payload))
}

func TestConn_WriteClose(t *testing.T) {
	var wg = sync.WaitGroup{}
	wg.Add(3)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	clientHandler.onClose = func(socket *Conn, err error) {
		wg.Done()
	}
	clientHandler.onMessage = func(socket *Conn, message *Message) {
		wg.Done()
	}
	go server.ReadLoop()
	go client.ReadLoop()

	server.WriteMessage(OpcodeText, nil)
	server.WriteMessage(OpcodeText, []byte("hello"))
	server.WriteMessage(OpcodeCloseConnection, []byte{1})
	wg.Wait()
}
