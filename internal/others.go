package internal

import (
	"math"
	"net"
)

const (
	// PermessageDeflate 表示 WebSocket 扩展 "permessage-deflate"
	// PermessageDeflate represents the WebSocket extension "permessage-deflate"
	PermessageDeflate = "permessage-deflate"

	// ServerMaxWindowBits 表示服务器最大窗口位数的参数
	// ServerMaxWindowBits represents the parameter for the server's maximum window bits
	ServerMaxWindowBits = "server_max_window_bits"

	// ClientMaxWindowBits 表示客户端最大窗口位数的参数
	// ClientMaxWindowBits represents the parameter for the client's maximum window bits
	ClientMaxWindowBits = "client_max_window_bits"

	// ServerNoContextTakeover 表示服务器不进行上下文接管的参数
	// ServerNoContextTakeover represents the parameter for the server's no context takeover
	ServerNoContextTakeover = "server_no_context_takeover"

	// ClientNoContextTakeover 表示客户端不进行上下文接管的参数
	// ClientNoContextTakeover represents the parameter for the client's no context takeover
	ClientNoContextTakeover = "client_no_context_takeover"

	// EQ 表示等号 "="
	// EQ represents the equal sign "="
	EQ = "="
)

// Pair 表示一个键值对
// Pair represents a key-value pair
type Pair struct {
	// Key 表示键
	// Key represents the key
	Key string

	// Val 表示值
	// Val represents the value
	Val string
}

var (
	// SecWebSocketVersion 表示 WebSocket 版本的键值对
	// SecWebSocketVersion represents the key-value pair for WebSocket version
	SecWebSocketVersion = Pair{"Sec-WebSocket-Version", "13"}

	// SecWebSocketKey 表示 WebSocket 密钥的键值对
	// SecWebSocketKey represents the key-value pair for WebSocket key
	SecWebSocketKey = Pair{"Sec-WebSocket-Key", ""}

	// SecWebSocketExtensions 表示 WebSocket 扩展的键值对
	// SecWebSocketExtensions represents the key-value pair for WebSocket extensions
	SecWebSocketExtensions = Pair{"Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover"}

	// Connection 表示连接类型的键值对
	// Connection represents the key-value pair for connection type
	Connection = Pair{"Connection", "Upgrade"}

	// Upgrade 表示升级协议的键值对
	// Upgrade represents the key-value pair for upgrade protocol
	Upgrade = Pair{"Upgrade", "websocket"}

	// SecWebSocketAccept 表示 WebSocket 接受密钥的键值对
	// SecWebSocketAccept represents the key-value pair for WebSocket accept key
	SecWebSocketAccept = Pair{"Sec-WebSocket-Accept", ""}

	// SecWebSocketProtocol 表示 WebSocket 协议的键值对
	// SecWebSocketProtocol represents the key-value pair for WebSocket protocol
	SecWebSocketProtocol = Pair{"Sec-WebSocket-Protocol", ""}
)

// MagicNumber 是 WebSocket 握手过程中使用的魔术字符串
// MagicNumber is the magic string used during the WebSocket handshake
const MagicNumber = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	// ThresholdV1 是第一个版本的阈值，最大值为 125
	// ThresholdV1 is the threshold for the first version, with a maximum value of 125
	ThresholdV1 = 125

	// ThresholdV2 是第二个版本的阈值，最大值为 math.MaxUint16
	// ThresholdV2 is the threshold for the second version, with a maximum value of math.MaxUint16
	ThresholdV2 = math.MaxUint16

	// ThresholdV3 是第三个版本的阈值，最大值为 math.MaxUint64
	// ThresholdV3 is the threshold for the third version, with a maximum value of math.MaxUint64
	ThresholdV3 = math.MaxUint64
)

// NetConn 是一个网络连接接口，定义了一个返回 net.Conn 的方法
// NetConn is a network connection interface that defines a method returning a net.Conn
type NetConn interface {
	// NetConn 返回一个底层的 net.Conn 对象
	// NetConn returns an underlying net.Conn object
	NetConn() net.Conn
}
