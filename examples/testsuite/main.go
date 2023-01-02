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
	var upgrader = gws.Upgrader{CompressEnabled: true, MaxContentLength: 32 * 1024 * 1024}
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
					handler.OnPing(socket, msg)
				case gws.OpcodePong:
					handler.OnPong(socket, msg)
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

func (c *WebSocketHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	socket.WriteMessage(message.Typ(), message.Bytes())
}

func (c *WebSocketHandler) OnError(socket *gws.Conn, err error) {
	println("error: ", err.Error())
}

func (c *WebSocketHandler) OnPing(socket *gws.Conn, message *gws.Message) {
	socket.WritePong(message.Bytes())
	socket.SetDeadline(time.Now().Add(30 * time.Second))
	_ = message.Close()
}

func (c *WebSocketHandler) OnPong(socket *gws.Conn, message *gws.Message) {
	_ = message.Close()
}
