package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval         = 5 * time.Second  // 客户端心跳间隔
	HeartbeatWaitTimeout = 10 * time.Second // 心跳等待超时时间
)

//go:embed index.html
var html []byte

func main() {
	var handler = NewWebSocket()
	var upgrader = gws.NewUpgrader(handler, &gws.ServerOption{
		PermessageDeflate: gws.PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		},

		// 在querystring里面传入用户名
		// 把Sec-WebSocket-Key作为连接的key
		// 刷新页面的时候, 会触发上一个连接的OnClose/OnError事件, 这时候需要对比key并删除map里存储的连接
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			var name = r.URL.Query().Get("name")
			if name == "" {
				return false
			}
			session.Store("name", name)
			session.Store("websocketKey", r.Header.Get("Sec-WebSocket-Key"))
			return true
		},
	})

	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			log.Printf("Accept: " + err.Error())
			return
		}
		socket.ReadLoop()
	})

	http.HandleFunc("/index.html", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(html)
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("%+v", err)
	}
}

func MustLoad[T any](session gws.SessionStorage, key string) (v T) {
	if value, exist := session.Load(key); exist {
		v, _ = value.(T)
	}
	return
}

func NewWebSocket() *WebSocket {
	return &WebSocket{
		sessions: gws.NewConcurrentMap[string, *gws.Conn](16, 128),
	}
}

type WebSocket struct {
	sessions *gws.ConcurrentMap[string, *gws.Conn] // 使用内置的ConcurrentMap存储连接, 可以减少锁冲突
}

func (c *WebSocket) OnOpen(socket *gws.Conn) {
	name := MustLoad[string](socket.Session(), "name")
	if conn, ok := c.sessions.Load(name); ok {
		conn.WriteClose(1000, []byte("connection is replaced"))
	}
	_ = socket.SetDeadline(time.Now().Add(PingInterval + HeartbeatWaitTimeout))
	c.sessions.Store(name, socket)
	log.Printf("%s connected\n", name)
}

func (c *WebSocket) OnClose(socket *gws.Conn, err error) {
	name := MustLoad[string](socket.Session(), "name")
	sharding := c.sessions.GetSharding(name)
	sharding.Lock()
	defer sharding.Unlock()

	if conn, ok := sharding.Load(name); ok {
		key0 := MustLoad[string](socket.Session(), "websocketKey")
		if key1 := MustLoad[string](conn.Session(), "websocketKey"); key1 == key0 {
			sharding.Delete(name)
		}
	}

	log.Printf("onerror, name=%s, msg=%s\n", name, err.Error())
}

func (c *WebSocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + HeartbeatWaitTimeout))
	_ = socket.WriteString("pong")
}

func (c *WebSocket) OnPong(socket *gws.Conn, payload []byte) {}

type Input struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

func (c *WebSocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	// chrome websocket不支持ping方法, 所以在text frame里面模拟ping
	if b := message.Bytes(); len(b) == 4 && string(b) == "ping" {
		c.OnPing(socket, nil)
		return
	}

	var input = &Input{}
	_ = json.Unmarshal(message.Bytes(), input)
	if conn, ok := c.sessions.Load(input.To); ok {
		_ = conn.WriteMessage(gws.OpcodeText, message.Bytes())
	}
}
