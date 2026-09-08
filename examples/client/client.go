package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/lxzan/gws"
)

// WebSocket 客户端示例: 连接服务器并交互式收发消息
func main() {
	// 创建客户端连接, 启用 PermessageDeflate 压缩
	socket, _, err := gws.NewClient(new(WebSocket), &gws.ClientOption{
		Addr: "ws://127.0.0.1:3000/connect",
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},
	})
	if err != nil {
		log.Printf(err.Error())
		return
	}
	// 启动异步读取循环, 接收服务端消息
	go socket.ReadLoop()

	// 从标准输入读取消息并发送
	for {
		var text = ""
		fmt.Scanf("%s", &text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		socket.WriteString(text)
	}
}

// WebSocket 实现 gws.Event 接口, 处理各类事件回调
type WebSocket struct {
}

func (c *WebSocket) OnClose(socket *gws.Conn, err error) {
	fmt.Printf("onerror: err=%s\n", err.Error())
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	_ = socket.WriteString("hello, there is client")
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	fmt.Printf("recv: %s\n", message.Data.String())
}
