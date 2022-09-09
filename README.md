# gws
> event driven websocket framework

### Quick Start
```go
// main.go
var upgrader = websocket.Upgrader{
    ServerOptions: &websocket.ServerOptions{
        LogEnabled:      true,
        CompressEnabled: false,
    },
    CheckOrigin: func(r *websocket.Request) bool {
        return true
    },
}

http.HandleFunc("/ws/connect", func(writer http.ResponseWriter, request *http.Request) {
    upgrader.Upgrade(writer, request, nil, NewWebSocketHandler())
})

// handler.go
func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{}
}

type WebSocketHandler struct{}

func (c *WebSocketHandler) OnRecover(socket *websocket.Conn, exception interface{}) {}

func (c *WebSocketHandler) OnOpen(socket *websocket.Conn) {}

func (c *WebSocketHandler) OnMessage(socket *websocket.Conn, m *websocket.Message) {}

func (c *WebSocketHandler) OnClose(socket *websocket.Conn, code websocket.Code, reason []byte) {}

func (c *WebSocketHandler) OnError(socket *websocket.Conn, err error) {}

func (c *WebSocketHandler) OnPing(socket *websocket.Conn, m []byte) {}

func (c *WebSocketHandler) OnPong(socket *websocket.Conn, m []byte) {}


```

### Core
```go
type EventHandler interface {
	OnRecover(socket *Conn, exception interface{})
	OnOpen(socket *Conn)
	OnClose(socket *Conn, code Code, reason []byte)
	OnMessage(socket *Conn, m *Message)
	OnError(socket *Conn, err error)
	OnPing(socket *Conn, m []byte)
	OnPong(socket *Conn, m []byte)
}