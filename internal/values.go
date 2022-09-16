package internal

const PANIC_ABORT = "PANIC_ABORT"

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
	Bv10 = 1 << 10
	Bv12 = 1 << 12
	Bv16 = 1 << 16
)

// Add four bytes as specified in RFC
// Add final block to squelch unexpected EOF error from flate reader.
var FlateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

// buffer level
const (
	Lv1 = 125
	Lv2 = 1024
	Lv3 = 4 * 1024
	Lv4 = 64*1024 - 1
	Lv5 = 1024 * 1024
)
