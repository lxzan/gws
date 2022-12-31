package gws

import "errors"

type Opcode uint8

const (
	OpcodeContinuation    Opcode = 0x0
	OpcodeText            Opcode = 0x1
	OpcodeBinary          Opcode = 0x2
	OpcodeCloseConnection Opcode = 0x8
	OpcodePing            Opcode = 0x9
	OpcodePong            Opcode = 0xA
)

type EventHandler interface {
	OnOpen(socket *Conn)
	OnMessage(socket *Conn, m *Message)
	OnError(socket *Conn, err error)
	OnPing(socket *Conn, m []byte)
	OnPong(socket *Conn, m []byte)
}

var closeErrorMap = map[Code]string{
	CloseNormalClosure:     "close normal",
	CloseGoingAway:         "client going away",
	CloseProtocolError:     "protocol error",
	CloseUnsupported:       "unsupported data",
	CloseNoStatusReceived:  "no status",
	CloseAbnormalClosure:   "abnormal closure",
	CloseUnsupportedData:   "invalid payload data",
	ClosePolicyViolation:   "policy violation",
	CloseMessageTooLarge:   "message too large",
	CloseMissingExtension:  "mandatory extension missing",
	CloseInternalServerErr: "internal server error",
	CloseServiceRestart:    "server restarting",
	CloseTryAgainLater:     "try again later",
	CloseTLSHandshake:      "TLS handshake error",
}

var (
	ERR_CheckOrigin        = errors.New("check origin error")
	ERR_WebSocketHandshake = errors.New("websocket handshake error")
)

type Code uint16

const (
	// 正常关闭; 无论为何目的而创建, 该链接都已成功完成任务.
	CloseNormalClosure Code = 1000

	// 终端离开, 可能因为服务端错误, 也可能因为浏览器正从打开连接的页面跳转离开.
	CloseGoingAway Code = 1001

	// 由于协议错误而中断连接.
	CloseProtocolError Code = 1002

	// 由于接收到不允许的数据类型而断开连接 (如仅接收文本数据的终端接收到了二进制数据).
	CloseUnsupported Code = 1003

	// 保留. 表示没有收到预期的状态码.
	CloseNoStatusReceived Code = 1005

	// 保留. 用于期望收到状态码时连接非正常关闭 (也就是说, 没有发送关闭帧).
	CloseAbnormalClosure Code = 1006

	// 由于收到了格式不符的数据而断开连接 (如文本消息中包含了非 UTF-8 数据).
	CloseUnsupportedData Code = 1007

	// 由于收到不符合约定的数据而断开连接. 这是一个通用状态码, 用于不适合使用 1003 和 1009 状态码的场景.
	ClosePolicyViolation Code = 1008

	// 由于收到过大的数据帧而断开连接.
	CloseMessageTooLarge Code = 1009

	// 客户端期望服务器商定一个或多个拓展, 但服务器没有处理, 因此客户端断开连接.
	CloseMissingExtension Code = 1010

	// 客户端由于遇到没有预料的情况阻止其完成请求, 因此服务端断开连接.
	CloseInternalServerErr Code = 1011

	// 服务器由于重启而断开连接. [Ref]
	CloseServiceRestart Code = 1012

	// 服务器由于临时原因断开连接, 如服务器过载因此断开一部分客户端连接. [Ref]
	CloseTryAgainLater Code = 1013

	// 保留. 表示连接由于无法完成 TLS 握手而关闭 (例如无法验证服务器证书).
	CloseTLSHandshake Code = 1015
)

func (c Code) Uint16() uint16 {
	return uint16(c)
}

func (c Code) Bytes() []byte {
	return []byte{uint8(c >> 8), uint8(c << 8 >> 8)}
}

func (c Code) Error() string {
	return "gws: " + closeErrorMap[c]
}
