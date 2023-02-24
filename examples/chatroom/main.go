package main

import (
	_ "embed"
	"encoding/json"
	"github.com/lxzan/gws"
	"log"
	"net/http"
	"time"
)

const PingInterval = 15 * time.Second // 客户端心跳间隔

//go:embed index.html
var html []byte

func main() {
	var handler = NewWebSocket()
	var upgrader = gws.NewUpgrader(func(c *gws.Upgrader) {
		c.CompressEnabled = true
		c.EventHandler = handler

		// 在querystring里面传入用户名
		// 把Sec-WebSocket-Key作为连接的key
		// 刷新页面的时候, 会触发上一个连接的OnClose/OnError事件, 这时候需要对比key并删除map里存储的连接
		c.CheckOrigin = func(r *gws.Request) bool {
			var name = r.URL.Query().Get("name")
			if name == "" {
				return false
			}
			r.SessionStorage.Store("name", name)
			r.SessionStorage.Store("key", r.Header.Get("Sec-WebSocket-Key"))
			return true
		}
	})

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Accept(writer, request)
		if err != nil {
			log.Printf("Accept: " + err.Error())
			return
		}
		socket.Listen()
	})

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(html)
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("%+v", err)
	}
}

func NewWebSocket() *WebSocket {
	return &WebSocket{sessions: gws.NewConcurrentMap(16)}
}

type WebSocket struct {
	sessions *gws.ConcurrentMap // 使用内置的ConcurrentMap存储连接, 可以减少锁冲突
}

func (c *WebSocket) getName(socket *gws.Conn) string {
	name, _ := socket.SessionStorage.Load("name")
	return name.(string)
}

func (c *WebSocket) getKey(socket *gws.Conn) string {
	name, _ := socket.SessionStorage.Load("key")
	return name.(string)
}

// 根据用户名获取WebSocket连接
func (c *WebSocket) GetSocket(name string) (*gws.Conn, bool) {
	if v0, ok0 := c.sessions.Load(name); ok0 {
		if v1, ok1 := v0.(*gws.Conn); ok1 {
			return v1, true
		}
	}
	return nil, false
}

// RemoveSocket 移除WebSocket连接
func (c *WebSocket) RemoveSocket(socket *gws.Conn) {
	name := c.getName(socket)
	key := c.getKey(socket)
	if mSocket, ok := c.GetSocket(name); ok {
		if mKey := c.getKey(mSocket); mKey == key {
			c.sessions.Delete(name)
		}
	}
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	name := c.getName(socket)
	if v, ok := c.sessions.Load(name); ok {
		var conn = v.(*gws.Conn)
		conn.Close(1000, []byte("connection replaced"))
	}
	socket.SetDeadline(time.Now().Add(3 * PingInterval))
	c.sessions.Store(name, socket)
	log.Printf("%s connected\n", name)
}

func (c *WebSocket) OnError(socket *gws.Conn, err error) {
	name := c.getName(socket)
	c.RemoveSocket(socket)
	log.Printf("onerror, name=%s, msg=%s\n", name, err.Error())
}

func (c *WebSocket) OnClose(socket *gws.Conn, code uint16, reason []byte) {
	name := c.getName(socket)
	c.RemoveSocket(socket)
	log.Printf("onclose, name=%s, code=%d, msg=%s\n", name, code, string(reason))
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

type Input struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	// chrome websocket不支持ping方法, 所以在text frame里面模拟ping
	if b := message.Data.Bytes(); len(b) == 4 && string(b) == "ping" {
		socket.Write(gws.OpcodeText, []byte("pong"))
		socket.SetDeadline(time.Now().Add(3 * PingInterval))
		return
	}

	var input = &Input{}
	_ = json.Unmarshal(message.Data.Bytes(), input)
	if v, ok := c.sessions.Load(input.To); ok {
		v.(*gws.Conn).Write(gws.OpcodeText, message.Data.Bytes())
	}
}
