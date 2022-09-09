package websocket

import "errors"

type Opcode uint8

const (
	Opcode_Continuation    Opcode = 0x0
	Opcode_Text            Opcode = 0x1
	Opcode_Binary          Opcode = 0x2
	Opcode_CloseConnection Opcode = 0x8
	Opcode_Ping            Opcode = 0x9
	Opcode_Pong            Opcode = 0xA
)

type EventHandler interface {
	OnRecover(socket *Conn, exception interface{})
	OnConnect(socket *Conn)
	OnMessage(socket *Conn, m *Message)
	OnClose(socket *Conn, code Code, reason []byte)
	OnError(socket *Conn, err error)
	OnPing(socket *Conn, m []byte)
	OnPong(socket *Conn, m []byte)
}

var closeErrorMap = map[Code]string{
	CloseNormalClosure:           "normal",
	CloseGoingAway:               "client/server going away",
	CloseProtocolError:           "protocol error",
	CloseUnsupportedData:         "unsupported data",
	CloseNoStatusReceived:        "no status",
	CloseAbnormalClosure:         "abnormal closure",
	CloseInvalidFramePayloadData: "invalid payload data",
	ClosePolicyViolation:         "policy violation",
	CloseMessageTooBig:           "message too big",
	CloseMandatoryExtension:      "mandatory extension missing",
	CloseInternalServerErr:       "internal server error",
	CloseServiceRestart:          "server restarting",
	CloseTryAgainLater:           "try again later",
	CloseTLSHandshake:            "TLS handshake error",
}

var (
	ERR_CheckOrigin        = errors.New("check origin error")
	ERR_WebSocketHandshake = errors.New("websocket handshake error")
	ERR_DISCONNECT         = errors.New("ERR_DISCONNECT")
)

type Code uint16

const (
	CloseNormalClosure           Code = 1000
	CloseGoingAway               Code = 1001
	CloseProtocolError           Code = 1002
	CloseUnsupportedData         Code = 1003
	CloseNoStatusReceived        Code = 1005
	CloseAbnormalClosure         Code = 1006
	CloseInvalidFramePayloadData Code = 1007
	ClosePolicyViolation         Code = 1008
	CloseMessageTooBig           Code = 1009
	CloseMandatoryExtension      Code = 1010
	CloseInternalServerErr       Code = 1011
	CloseServiceRestart          Code = 1012
	CloseTryAgainLater           Code = 1013
	CloseTLSHandshake            Code = 1015
)

func (c Code) Bytes() []byte {
	return []byte{uint8(c >> 8), uint8(c << 8 >> 8)}
}

func (c Code) Error() string {
	return "websocket close: " + closeErrorMap[c]
}
