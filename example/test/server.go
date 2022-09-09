package main

import (
	//"github.com/gorilla/websocket"
	"os"
	"runtime"

	"github.com/lxzan/websocket"
	"net/http"
	_ "net/http/pprof"
)

var content = `
FIN, 长度为 1 比特, 该标志位用于指示当前的 frame 是消息的最后一个分段, 因为 WebSocket 支持将长消息切分为若干个 frame 发送, 切分以后, 除了最后一个 frame, 前面的 frame 的 FIN 字段都为 0, 最后一个 frame 的 FIN 字段为 1, 当然, 若消息没有分段, 那么一个 frame 便包含了完成的消息, 此时其 FIN 字段值为 1
RSV 1 ~ 3, 这三个字段为保留字段, 只有在 WebSocket 扩展时用, 若不启用扩展, 则该三个字段应置为 1, 若接收方收到 RSV 1 ~ 3 不全为 0 的 frame, 并且双方没有协商使用 WebSocket 协议扩展, 则接收方应立即终止 WebSocket 连接
RSV 1 ~ 3, 这三个字段为保留字段, 只有在 WebSocket 扩展时用, 若不启用扩展, 则该三个字段应置为 1, 若接收方收到 RSV 1 ~ 3 不全为 0 的 frame, 并且双方没有协商使用 WebSocket 协议扩展, 则接收方应立即终止 WebSocket 连接
RSV 1 ~ 3, 这三个字段为保留字段, 只有在 WebSocket 扩展时用, 若不启用扩展, 则该三个字段应置为 1, 若接收方收到 RSV 1 ~ 3 不全为 0 的 frame, 并且双方没有协商使用 WebSocket 协议扩展, 则接收方应立即终止 WebSocket 连接
`

func main() {
	runtime.GOMAXPROCS(8)

	websocket.SetConfig(&websocket.Config{Compress: true})

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *websocket.Request) bool {
			return true
		},
	}

	//1, 3, 5, 4, 2
	//upgrader.Use(
	//	func(socket *websocket.Conn, msg *websocket.Message) {
	//		println("step 1")
	//		msg.Next(socket)
	//		println("step 2")
	//	}, func(socket *websocket.Conn, msg *websocket.Message) {
	//		println("step 3")
	//		//msg.Next(socket)
	//		msg.Abort(socket)
	//		return
	//		println("step 4")
	//	},
	//)

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		content, _ := os.ReadFile(os.Getenv("STATIC_DIR") + "/index.html")
		writer.Write(content)
	})

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		upgrader.Upgrade(writer, request, nil, NewWebSocketHandler())
	})

	http.ListenAndServe(":3000", nil)
}
