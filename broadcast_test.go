package gws

import (
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestNewBroadcaster(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var handler = &broadcastHandler{sockets: &sync.Map{}, wg: &sync.WaitGroup{}}
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{
			CompressEnabled: true,
		})

		app.OnRequest = func(socket *Conn, request *http.Request) {
			handler.sockets.Store(socket, struct{}{})
		}

		go func() {
			if err := app.Run(addr); err != nil {
				as.NoError(err)
				return
			}
		}()

		time.Sleep(500 * time.Millisecond)

		var count = 100
		for i := 0; i < count; i++ {
			compress := i%2 == 0
			client, _, err := NewClient(handler, &ClientOption{Addr: "ws://" + addr, CompressEnabled: compress})
			if err != nil {
				as.NoError(err)
				return
			}
			_ = client.WritePing(nil)
			go client.ReadLoop()
		}

		handler.wg.Add(count)
		var b = NewBroadcaster(OpcodeText, internal.AlphabetNumeric.Generate(1000))
		handler.sockets.Range(func(key, value any) bool {
			_ = b.Broadcast(key.(*Conn))
			return true
		})
		b.Release()
		handler.wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var handler = &broadcastHandler{sockets: &sync.Map{}, wg: &sync.WaitGroup{}}
		var addr = "127.0.0.1:" + nextPort()
		app := NewServer(new(BuiltinEventHandler), &ServerOption{
			CompressEnabled:     true,
			WriteMaxPayloadSize: 1000,
		})

		app.OnRequest = func(socket *Conn, request *http.Request) {
			handler.sockets.Store(socket, struct{}{})
		}

		go func() {
			if err := app.Run(addr); err != nil {
				as.NoError(err)
				return
			}
		}()

		time.Sleep(500 * time.Millisecond)

		var count = 100
		for i := 0; i < count; i++ {
			compress := i%2 == 0
			client, _, err := NewClient(handler, &ClientOption{Addr: "ws://" + addr, CompressEnabled: compress})
			if err != nil {
				as.NoError(err)
				return
			}
			go client.ReadLoop()
		}

		var b = NewBroadcaster(OpcodeText, testdata)
		handler.sockets.Range(func(key, value any) bool {
			if err := b.Broadcast(key.(*Conn)); err == nil {
				handler.wg.Add(1)
			}
			return true
		})
		time.Sleep(500 * time.Millisecond)
		b.Release()
		handler.wg.Wait()
	})
}

type broadcastHandler struct {
	wg      *sync.WaitGroup
	sockets *sync.Map
}

func (b broadcastHandler) OnOpen(socket *Conn) {
}

func (b broadcastHandler) OnClose(socket *Conn, err error) {
}

func (b broadcastHandler) OnPing(socket *Conn, payload []byte) {
}

func (b broadcastHandler) OnPong(socket *Conn, payload []byte) {
}

func (b broadcastHandler) OnMessage(socket *Conn, message *Message) {
	defer message.Close()
	b.wg.Done()
}
