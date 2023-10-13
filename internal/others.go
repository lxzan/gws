package internal

import (
	"math"
	"net"
)

const PermessageDeflate = "permessage-deflate"

type Pair struct {
	Key string
	Val string
}

var (
	SecWebSocketVersion    = Pair{"Sec-WebSocket-Version", "13"}
	SecWebSocketKey        = Pair{"Sec-WebSocket-Key", ""}
	SecWebSocketExtensions = Pair{"Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover"}
	Connection             = Pair{"Connection", "Upgrade"}
	Upgrade                = Pair{"Upgrade", "websocket"}
	SecWebSocketAccept     = Pair{"Sec-WebSocket-Accept", ""}
	SecWebSocketProtocol   = Pair{"Sec-WebSocket-Protocol", ""}
)

const MagicNumber = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	ThresholdV1 = 125
	ThresholdV2 = math.MaxUint16
	ThresholdV3 = math.MaxUint64
)

type NetConn interface {
	NetConn() net.Conn
}
