package main

import (
	"context"
	"errors"
	"flag"
	"github.com/lxzan/gws"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

var directory string

func main() {
	flag.StringVar(&directory, "d", "./", "directory")
	flag.Parse()

	var upgrader = gws.Upgrader{}

	var handler = NewWebSocketHandler()
	ctx, cancel := context.WithCancel(context.Background())

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(ctx, writer, request)
		if err != nil {
			return
		}

		handler.OnOpen(socket)
		for {
			select {
			case <-ctx.Done():
				handler.OnError(socket, gws.CloseServiceRestart)
				return
			case msg := <-socket.Read():
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

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		d, _ := filepath.Abs(directory)
		content, _ := os.ReadFile(d + "/index.html")
		writer.Write(content)
	})

	go http.ListenAndServe(":3000", nil)

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	time.Sleep(3 * time.Second)
}
