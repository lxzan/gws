package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"unicode/utf8"

	"github.com/lxzan/gws"
)

func main() {
	s1 := gws.NewServer(&Handler{Sync: true}, &gws.ServerOption{
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s2 := gws.NewServer(&Handler{Sync: false}, &gws.ServerOption{
		ParallelEnabled: true,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s3 := gws.NewServer(&Handler{Sync: true}, &gws.ServerOption{
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
		},
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s4 := gws.NewServer(&Handler{Sync: false}, &gws.ServerOption{
		ParallelEnabled: true,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: false,
			ClientContextTakeover: false,
		},
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})

	s5 := newNextReaderServer()

	go func() {
		log.Panic(s1.Run(":8000"))
	}()

	go func() {
		log.Panic(s2.Run(":8001"))
	}()

	go func() {
		log.Panic(s3.Run(":8002"))
	}()

	go func() {
		log.Panic(s4.Run(":8003"))
	}()

	log.Panic(s5.Run(":8004"))
}

type Handler struct {
	gws.BuiltinEventHandler
	Sync bool
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	if c.Sync {
		_ = socket.WriteMessage(message.Opcode, message.Bytes())
		_ = message.Close()
	} else {
		socket.WriteAsync(message.Opcode, message.Bytes(), func(err error) { _ = message.Close() })
	}
}

func newNextReaderServer() *gws.Server {
	server := gws.NewServer(&Handler{Sync: true}, &gws.ServerOption{
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
	})
	server.OnRequest = func(conn net.Conn, br *bufio.Reader, r *http.Request) {
		socket, err := server.GetUpgrader().UpgradeFromConn(conn, br, r)
		if err != nil {
			server.OnError(conn, err)
			return
		}
		nextReaderEcho(socket)
	}
	return server
}

func nextReaderEcho(socket *gws.Conn) {
	for {
		opcode, r, err := socket.NextReader()
		if err != nil {
			return
		}
		payload, err := io.ReadAll(r)
		if err != nil {
			return
		}
		if opcode == gws.OpcodeText {
			if !utf8.Valid(payload) {
				_ = socket.WriteClose(1007, nil)
				return
			}
		}
		if err := socket.WriteMessage(opcode, payload); err != nil {
			return
		}
	}
}
