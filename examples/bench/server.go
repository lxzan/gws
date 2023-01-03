package main

import (
	"context"
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

	var upgrader = gws.Upgrader{MaxConcurrency: 1}

	var handler = NewWebSocket()
	ctx, cancel := context.WithCancel(context.Background())

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = upgrader.Upgrade(ctx, writer, request, handler)
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
	cancel()
	time.Sleep(100 * time.Millisecond)
}
