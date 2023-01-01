package main

import (
	"context"
	"errors"
	"github.com/lxzan/gws"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	var upgrader = gws.Upgrader{}
	var handler = new(WebSocketHandler)
	ctx, cancel := context.WithCancel(context.Background())

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(ctx, writer, request)
		if err != nil {
			return
		}
		defer socket.Close()

		handler.OnOpen(socket)
		for {
			select {
			case <-ctx.Done():
				handler.OnError(socket, gws.CloseServiceRestart)
				return
			case msg := <-socket.ReadMessage():
				if err := msg.Err(); err != nil {
					handler.OnError(socket, err)
					return
				}

				switch msg.Typ() {
				case gws.OpcodeText, gws.OpcodeBinary:
					handler.OnMessage(socket, msg)
				case gws.OpcodePing:
					handler.OnPing(socket, msg.Bytes())
				case gws.OpcodePong:
					handler.OnPong(socket, msg.Bytes())
				default:
					handler.OnError(socket, errors.New("unexpected opcode: "+strconv.Itoa(int(msg.Typ()))))
					return
				}
			}
		}
	})

	go http.ListenAndServe(":3000", nil)

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	time.Sleep(100 * time.Millisecond)
}

type WebSocketHandler struct{}

func (c *WebSocketHandler) OnOpen(socket *gws.Conn) {
	println("connected")
}

func (c *WebSocketHandler) OnMessage(socket *gws.Conn, m *gws.Message) {
	defer m.Close()
	println(string(m.Bytes()))
}

func (c *WebSocketHandler) OnError(socket *gws.Conn, err error) {
	println(err.Error())
}

func (c *WebSocketHandler) OnPing(socket *gws.Conn, m []byte) {
	println("onping")
}

func (c *WebSocketHandler) OnPong(socket *gws.Conn, m []byte) {
	println("onpong")
}
