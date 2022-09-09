package internal

// websocket header keys
const (
	SecWebSocketVersion    = "Sec-WebSocket-Version"
	SecWebSocketKey        = "Sec-WebSocket-Key"
	Connection             = "Connection"
	Upgrade                = "Upgrade"
	SecWebSocketExtensions = "Sec-WebSocket-Extensions"
)

// websocket header values
const (
	SecWebSocketVersion_Value = "13"
	Connection_Value          = "Upgrade"
	Upgrade_Value             = "websocket"
)

const (
	MagicNumber     = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	FrameHeaderSize = 14
)

const (
	Bv4  = 1 << 4
	Bv5  = 1 << 5
	Bv6  = 1 << 6
	Bv7  = 1 << 7
	Bv8  = 1 << 8
	Bv10 = 1 << 10 // 1KB
	Bv12 = 1 << 12 // 4KB
	Bv16 = 1 << 16 // 64KB
)

const (
	PayloadSizeLv1 = 125     // 125B
	PayloadSizeLv2 = 1 << 16 // 64KB
	PayloadSizeLv3 = 1 << 20 // 1MB
)

// Add four bytes as specified in RFC
// Add final block to squelch unexpected EOF error from flate reader.
var FlateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

var (
	PingFrame = []byte{137, 1, 48}
	PongFrame = []byte{138, 1, 49}
)
