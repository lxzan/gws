package gws

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

type webSocketMocker struct {
	sync.Mutex
	onMessage func(socket *Conn, message *Message)
	onPing    func(socket *Conn, payload []byte)
	onPong    func(socket *Conn, payload []byte)
	onClose   func(socket *Conn, err error)
}

func (c *webSocketMocker) reset(socket *Conn, reader *bytes.Buffer, writer *bytes.Buffer) {
	reader.Reset()
	writer.Reset()
	socket.br.Reset(reader)
	atomic.StoreUint32(&socket.closed, 0)
}

func (c *webSocketMocker) OnOpen(socket *Conn) {
}

func (c *webSocketMocker) OnClose(socket *Conn, err error) {
	if c.onClose != nil {
		c.onClose(socket, err)
	}
}

func (c *webSocketMocker) OnPing(socket *Conn, payload []byte) {
	if c.onPing != nil {
		c.onPing(socket, payload)
	}
}

func (c *webSocketMocker) OnPong(socket *Conn, payload []byte) {
	if c.onPong != nil {
		c.onPong(socket, payload)
	}
}

func (c *webSocketMocker) OnMessage(socket *Conn, message *Message) {
	if c.onMessage != nil {
		c.onMessage(socket, message)
	}
}

func TestOthers(t *testing.T) {
	conn, _ := net.Pipe()
	upgrader := NewUpgrader(new(BuiltinEventHandler), nil)
	socket := &Conn{
		conn:    conn,
		handler: new(webSocketMocker),
		config:  upgrader.option.getConfig(),
	}
	socket.SetDeadline(time.Time{})
	socket.SetReadDeadline(time.Time{})
	socket.SetWriteDeadline(time.Time{})
	socket.LocalAddr()
	socket.NetConn()
	socket.RemoteAddr()

	var as = assert.New(t)
	var fh = frameHeader{}
	fh.SetMask()
	var maskKey [4]byte
	copy(maskKey[:4], internal.AlphabetNumeric.Generate(4))
	fh.SetMaskKey(10, maskKey)
	as.Equal(true, fh.GetMask())
	as.Equal(string(maskKey[:4]), string(fh.GetMaskKey()))
	return
}

func TestConn_Close(t *testing.T) {
	conn, _ := net.Pipe()
	var socket = &Conn{conn: conn, closed: 1}
	assert.Error(t, socket.SetDeadline(time.Time{}))
	assert.Error(t, socket.SetReadDeadline(time.Time{}))
	assert.Error(t, socket.SetWriteDeadline(time.Time{}))
}

func TestConn_SubProtocol(t *testing.T) {
	conn := new(Conn)
	conn.SubProtocol()
}

func TestConn_EmitClose(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{CheckUtf8Enabled: true}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		clientHandler.onClose = func(socket *Conn, err error) {
			if err.(*CloseError).Code == internal.CloseProtocolError.Uint16() {
				wg.Done()
			}
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		server.emitClose(bytes.NewBuffer(internal.StatusCode(500).Bytes()))
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{CheckUtf8Enabled: true}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		clientHandler.onClose = func(socket *Conn, err error) {
			if err.(*CloseError).Code == 4000 {
				wg.Done()
			}
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		server.emitClose(bytes.NewBuffer(internal.StatusCode(4000).Bytes()))
		wg.Wait()
	})
}

func TestConn_EmitError(t *testing.T) {
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	var wg = &sync.WaitGroup{}
	wg.Add(1)
	clientHandler.onClose = func(socket *Conn, err error) {
		wg.Done()
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go client.ReadLoop()
	err := errors.New(string(internal.AlphabetNumeric.Generate(500)))
	server.emitError(err)
	wg.Wait()
}
