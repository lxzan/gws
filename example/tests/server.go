package main

import (
	"flag"
	"github.com/lxzan/gws"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var directory string

func main() {
	flag.StringVar(&directory, "d", "./", "directory")
	flag.Parse()

	const bufferSize = 16 * 1024
	var upgrader = gws.Upgrader{
		ServerOptions: &gws.ServerOptions{
			LogEnabled:      true,
			CompressEnabled: false,
			Concurrency:     16,
			WriteBufferSize: bufferSize,
			ReadBufferSize:  bufferSize,
			ReadTimeout:     time.Hour,
		},
		CheckOrigin: func(r *gws.Request) bool {
			return true
		},
	}

	//upgrader.Use(gws.RateLimiter(time.Minute, 10))

	var handler = NewWebSocketHandler()

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		upgrader.Upgrade(writer, request, handler)
	})

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		d, _ := filepath.Abs(directory)
		content, _ := os.ReadFile(d + "/index.html")
		writer.Write(content)
	})

	go http.ListenAndServe(":3000", nil)

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
