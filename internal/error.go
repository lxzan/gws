package internal

// closeErrorMap 将状态码映射到错误信息
// map status codes to error messages
var closeErrorMap = map[StatusCode]string{
	0:                     "empty code",
	CloseNormalClosure:    "close normal",
	CloseGoingAway:        "client going away",
	CloseProtocolError:    "protocol error",
	CloseUnsupported:      "unsupported data",
	CloseNoStatusReceived: "no status",
	CloseAbnormalClosure:  "abnormal closure",
	CloseUnsupportedData:  "invalid payload data",
	ClosePolicyViolation:  "policy violation",
	CloseMessageTooLarge:  "message too large",
	CloseMissingExtension: "mandatory extension missing",
	CloseInternalErr:      "internal error",
	CloseServiceRestart:   "server restarting",
	CloseTryAgainLater:    "try again later",
	CloseTLSHandshake:     "TLS handshake error",
}

// StatusCode WebSocket错误码
// websocket error code
type StatusCode uint16

const (
	// CloseNormalClosure 正常关闭; 无论为何目的而创建, 该链接都已成功完成任务.
	CloseNormalClosure StatusCode = 1000

	// CloseGoingAway 终端离开, 可能因为服务端错误, 也可能因为浏览器正从打开连接的页面跳转离开.
	CloseGoingAway StatusCode = 1001

	// CloseProtocolError 由于协议错误而中断连接.
	CloseProtocolError StatusCode = 1002

	// CloseUnsupported 由于接收到不允许的数据类型而断开连接 (如仅接收文本数据的终端接收到了二进制数据).
	CloseUnsupported StatusCode = 1003

	// CloseNoStatusReceived 保留. 表示没有收到预期的状态码.
	CloseNoStatusReceived StatusCode = 1005

	// CloseAbnormalClosure 保留. 用于期望收到状态码时连接非正常关闭 (也就是说, 没有发送关闭帧).
	CloseAbnormalClosure StatusCode = 1006

	// CloseUnsupportedData 由于收到了格式不符的数据而断开连接 (如文本消息中包含了非 UTF-8 数据).
	CloseUnsupportedData StatusCode = 1007

	// ClosePolicyViolation 由于收到不符合约定的数据而断开连接. 这是一个通用状态码, 用于不适合使用 1003 和 1009 状态码的场景.
	ClosePolicyViolation StatusCode = 1008

	// CloseMessageTooLarge 由于收到过大的数据帧而断开连接.
	CloseMessageTooLarge StatusCode = 1009

	// CloseMissingExtension 客户端期望服务器商定一个或多个拓展, 但服务器没有处理, 因此客户端断开连接.
	CloseMissingExtension StatusCode = 1010

	// CloseInternalErr 客户端由于遇到没有预料的情况阻止其完成请求, 因此服务端断开连接.
	CloseInternalErr StatusCode = 1011

	// CloseServiceRestart 服务器由于重启而断开连接. [Ref]
	CloseServiceRestart StatusCode = 1012

	// CloseTryAgainLater 服务器由于临时原因断开连接, 如服务器过载因此断开一部分客户端连接. [Ref]
	CloseTryAgainLater StatusCode = 1013

	// CloseTLSHandshake 保留. 表示连接由于无法完成 TLS 握手而关闭 (例如无法验证服务器证书).
	CloseTLSHandshake StatusCode = 1015
)

func (c StatusCode) Uint16() uint16 {
	return uint16(c)
}

func (c StatusCode) Bytes() []byte {
	if c == 0 {
		return []byte{}
	}
	return []byte{uint8(c >> 8), uint8(c << 8 >> 8)}
}

func (c StatusCode) Error() string {
	return "gws: " + closeErrorMap[c]
}

func NewError(code StatusCode, err error) *Error {
	return &Error{Code: code, Err: err}
}

type Error struct {
	Err  error      // 错误信息
	Code StatusCode // 状态码
}

func (c *Error) Error() string {
	return c.Err.Error()
}

// Errors 依次执行传入的函数，返回第一个遇到的错误
// executes the passed functions in sequence and returns the first encountered error
func Errors(funcs ...func() error) error {
	for _, f := range funcs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
