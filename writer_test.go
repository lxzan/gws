package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
)

func testWrite(c *Conn, fin bool, opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var useCompress = c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressThreshold
	if useCompress {
		var buf = bytes.NewBufferString("")
		err := c.compressor.Compress(payload, buf)
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
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{WriteMaxPayloadSize: 16}
	var clientOption = &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.Listen()
	go client.Listen()
	var err = server.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
	assert.Error(t, err)
}

func TestWriteClose(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{}
	var clientOption = &ClientOption{}

	var wg = sync.WaitGroup{}
	wg.Add(1)
	serverHandler.onError = func(socket *Conn, err error) {
		as.Error(err)
		wg.Done()
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.Listen()
	go client.Listen()
	server.WriteClose(1000, []byte("goodbye"))
	wg.Wait()
}

func TestConn_WriteAsyncError(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{WriteAsyncCap: 1}
		var clientOption = &ClientOption{}
		server, _ := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		server.WriteAsync(OpcodeText, nil)
		server.WriteAsync(OpcodeText, nil)
		err := server.WriteAsync(OpcodeText, nil)
		as.Equal(internal.ErrAsyncIOCapFull, err)
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		server, _ := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		server.closed = 1
		err := server.WriteAsync(OpcodeText, nil)
		as.Equal(internal.ErrConnClosed, err)
	})
}

func TestConn_WriteInvalidUTF8(t *testing.T) {
	var as = assert.New(t)
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.Listen()
	go client.Listen()
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
	clientHandler.onClose = func(socket *Conn, code uint16, reason []byte) {
		wg.Done()
	}
	clientHandler.onMessage = func(socket *Conn, message *Message) {
		wg.Done()
	}
	go server.Listen()
	go client.Listen()

	//var payload = internal.CloseGoingAway.Bytes()
	//payload = append(payload, "goodbye"...)
	server.WriteMessage(OpcodeText, nil)
	server.WriteMessage(OpcodeText, []byte("hello"))
	server.WriteMessage(OpcodeCloseConnection, []byte{1})
	wg.Wait()
}
