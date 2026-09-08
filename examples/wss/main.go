package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/lxzan/gws"
)

// WSS (TLS) 安全连接示例: 使用证书启动 HTTPS/WSS 服务
var dir string

func init() {
	// 通过 -d 参数指定证书目录
	flag.StringVar(&dir, "d", "", "cert directory")
	flag.Parse()

	d, err := filepath.Abs(dir)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	dir = d
}

func main() {
	srv := gws.NewServer(new(Websocket), nil)

	// 启动 TLS 服务, 客户端通过 wss://host:8443/ 连接
	if err := srv.RunTLS(":8443", dir+"/server.crt", dir+"/server.pem"); err != nil {
		log.Panicln(err.Error())
	}
}

// Websocket 嵌入 BuiltinEventHandler 获得默认事件处理, 按需覆写方法
type Websocket struct {
	gws.BuiltinEventHandler
}

func (c *Websocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Websocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
