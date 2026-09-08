package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lxzan/gws"
)

// 服务端推送示例: 从标准输入读取消息并广播给所有已连接客户端
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
		// 以 Sec-WebSocket-Key 作为唯一标识存储连接
		websocketKey := request.Header.Get("Sec-WebSocket-Key")
		socket.Session().Store("websocketKey", websocketKey)
		h.conns.Store(websocketKey, socket)
		go func() {
			socket.ReadLoop()
		}()
	})

	// 后台启动 HTTP 服务
	go func() {
		if err := http.ListenAndServe(":8000", nil); err != nil {
			return
		}
	}()

	// 从标准输入读取消息并广播
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

// Broadcast 使用 Broadcaster 向所有连接广播消息, 避免重复序列化
func (c *Handler) Broadcast(msg string) {
	var b = gws.NewBroadcaster(gws.OpcodeText, []byte(msg))
	c.conns.Range(func(key string, conn *gws.Conn) bool {
		_ = b.Broadcast(conn, nil)
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
