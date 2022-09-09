package main

import (
	"flag"
	websocket "github.com/lxzan/gws"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
)

var directory string

func main() {
	flag.StringVar(&directory, "d", "./", "directory")
	flag.Parse()

	var upgrader = websocket.Upgrader{
		ServerOptions: &websocket.ServerOptions{
			LogEnabled:      true,
			CompressEnabled: false,
		},
		CheckOrigin: func(r *websocket.Request) bool {
			return true
		},
	}

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		upgrader.Upgrade(writer, request, nil, NewWebSocketHandler())
	})

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		d, _ := filepath.Abs(directory)
		content, _ := os.ReadFile(d + "/index.html")
		writer.Write(content)
	})

	http.ListenAndServe(":3000", nil)
}
