package main

import (
	"flag"
	websocket "github.com/lxzan/gws"
	"net/http"
	_ "net/http/pprof"
)

var directory string

func main() {
	flag.StringVar(&directory, "d", "./", "directory")
	flag.Parse()

	websocket.SetConfig(&websocket.Config{Compress: true, LogEnabled: true})

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

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		upgrader.Upgrade(writer, request, nil, NewWebSocketHandler())
	})

	http.ListenAndServe(":3000", http.FileServer(http.Dir(directory)))
}
