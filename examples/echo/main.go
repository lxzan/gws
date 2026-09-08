package main

import (
	"log"
	"net/http"

	"github.com/lxzan/gws"
)

// Echo 回声服务示例: 收到消息后原样返回, 最简单的 WebSocket 服务端
func main() {
	upgrader := gws.NewUpgrader(&Handler{}, &gws.ServerOption{
		CheckUtf8Enabled: true,
		Recovery:         gws.Recovery,
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			return
		}
		// 每个连接启动独立协程读取消息
		go func() {
			socket.ReadLoop()
		}()
	})
	log.Panic(
		http.ListenAndServe(":8000", nil),
	)
}

// Handler 嵌入 BuiltinEventHandler 获得默认事件处理, 按需覆写方法
type Handler struct {
	gws.BuiltinEventHandler
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
