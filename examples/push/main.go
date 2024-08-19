package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lxzan/gws"
)

func main() {
	var h = &Handler{conns: gws.NewConcurrentMap[string, *gws.Conn]()}

	var upgrader = gws.NewUpgrader(h, &gws.ServerOption{
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			log.Println(err.Error())
			return
		}
		websocketKey := request.Header.Get("Sec-WebSocket-Key")
		socket.Session().Store("websocketKey", websocketKey)
		h.conns.Store(websocketKey, socket)
		go func() {
			socket.ReadLoop()
		}()
	})

	go func() {
		if err := http.ListenAndServe(":8000", nil); err != nil {
			return
		}
	}()

	for {
		var msg = ""
		if _, err := fmt.Scanf("%s\n", &msg); err != nil {
			log.Println(err.Error())
			return
		}
		h.Broadcast(msg)
	}
}

func getSession[T any](s gws.SessionStorage, key string) (val T) {
	if v, ok := s.Load(key); ok {
		val, _ = v.(T)
	}
	return
}

type Handler struct {
	gws.BuiltinEventHandler
	conns *gws.ConcurrentMap[string, *gws.Conn]
}

func (c *Handler) Broadcast(msg string) {
	var b = gws.NewBroadcaster(gws.OpcodeText, []byte(msg))
	c.conns.Range(func(key string, conn *gws.Conn) bool {
		_ = b.Broadcast(conn)
		return true
	})
	_ = b.Close()
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {
	websocketKey := getSession[string](socket.Session(), "websocketKey")
	c.conns.Delete(websocketKey)
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
}
