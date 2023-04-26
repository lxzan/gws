package main

import (
	"crypto/tls"
	"flag"
	"github.com/lxzan/gws"
	"log"
	"net"
	"path/filepath"
)

var dir string

func main() {
	flag.StringVar(&dir, "d", "", "cert directory")
	flag.Parse()

	basedir, err := filepath.Abs(dir)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	srv, err := gws.NewServer(new(Websocket), &gws.ServerOption{CheckUtf8Enabled: true})
	if err != nil {
		log.Printf(err.Error())
		return
	}

	srv.OnError = func(conn net.Conn, err error) {
		println(err.Error())
	}

	cert, err := tls.LoadX509KeyPair(basedir+"/server.crt", basedir+"/server.pem")
	if err != nil {
		log.Printf(err.Error())
		return
	}

	err = srv.RunTLS(":3000", &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"http/1.1"},
	})
	if err != nil {
		log.Panicln(err.Error())
	}
}

type Websocket struct {
	gws.BuiltinEventHandler
}

func (w Websocket) OnPing(socket *gws.Conn, payload []byte) {
	socket.WritePong(payload)
}

func (w Websocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
