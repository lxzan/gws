package internal

import "math"

// Add four bytes as specified in RFC
// Add final block to squelch unexpected EOF error from flate reader.
var FlateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

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
	ThresholdV1 = 125
	ThresholdV2 = math.MaxUint16
	ThresholdV3 = math.MaxUint64
)

// buffer level
const (
	Lv1 = 128
	Lv2 = 1024
	Lv3 = 4 * 1024
	Lv4 = 16 * 1024
	Lv5 = 64*1024 - 1
)
