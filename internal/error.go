package internal

// closeErrorMap 是一个映射，用于将状态码映射到错误信息
// closeErrorMap is a map used to map status codes to error messages
var closeErrorMap = map[StatusCode]string{
	// 空状态码
	// Empty status code
	0: "empty code",

	// 正常关闭; 无论为何目的而创建, 该链接都已成功完成任务.
	// Normal closure; the connection was closed successfully for a purpose.
	CloseNormalClosure: "close normal",

	// 终端离开, 可能因为服务端错误, 也可能因为浏览器正从打开连接的页面跳转离开.
	// The terminal is leaving, possibly due to a server error, or because the browser is leaving the page with an open connection.
	CloseGoingAway: "client going away",

	// 由于协议错误而中断连接.
	// The connection was terminated due to a protocol error.
	CloseProtocolError: "protocol error",

	// 由于接收到不允许的数据类型而断开连接 (如仅接收文本数据的终端接收到了二进制数据).
	// The connection was terminated due to receiving an unsupported data type (e.g., a terminal that only receives text data received binary data).
	CloseUnsupported: "unsupported data",

	// 表示没有收到预期的状态码.
	// Indicates that the expected status code was not received.
	CloseNoStatusReceived: "no status",

	// 用于期望收到状态码时连接非正常关闭 (也就是说, 没有发送关闭帧).
	// Used when the connection is abnormally closed when expecting a status code (i.e., no close frame was sent).
	CloseAbnormalClosure: "abnormal closure",

	// 由于收到了格式不符的数据而断开连接 (如文本消息中包含了非 UTF-8 数据).
	// The connection was terminated due to receiving data in an incorrect format (e.g., non-UTF-8 data in a text message).
	CloseUnsupportedData: "invalid payload data",

	// 由于违反策略而断开连接.
	// The connection was terminated due to a policy violation.
	ClosePolicyViolation: "policy violation",

	// 由于消息过大而断开连接.
	// The connection was terminated because the message was too large.
	CloseMessageTooLarge: "message too large",

	// 由于缺少必要的扩展而断开连接.
	// The connection was terminated due to a mandatory extension missing.
	CloseMissingExtension: "mandatory extension missing",

	// 由于内部服务器错误而断开连接.
	// The connection was terminated due to an internal server error.
	CloseInternalServerErr: "internal server error",

	// 由于服务器重启而断开连接.
	// The connection was terminated because the server is restarting.
	CloseServiceRestart: "server restarting",

	// 由于服务器过载或其他原因, 建议客户端稍后重试.
	// The connection was terminated due to server overload or other reasons, suggesting the client try again later.
	CloseTryAgainLater: "try again later",

	// 由于 TLS 握手失败而断开连接.
	// The connection was terminated due to a TLS handshake failure.
	CloseTLSHandshake: "TLS handshake error",
}

// StatusCode 类型定义为一个 uint16
// StatusCode type is defined as a uint16
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

	// CloseInternalServerErr 客户端由于遇到没有预料的情况阻止其完成请求, 因此服务端断开连接.
	CloseInternalServerErr StatusCode = 1011

	// CloseServiceRestart 服务器由于重启而断开连接. [Ref]
	CloseServiceRestart StatusCode = 1012

	// CloseTryAgainLater 服务器由于临时原因断开连接, 如服务器过载因此断开一部分客户端连接. [Ref]
	CloseTryAgainLater StatusCode = 1013

	// CloseTLSHandshake 保留. 表示连接由于无法完成 TLS 握手而关闭 (例如无法验证服务器证书).
	CloseTLSHandshake StatusCode = 1015
)

// Uint16 将 StatusCode 转换为 uint16
// Uint16 converts StatusCode to uint16
func (c StatusCode) Uint16() uint16 {
	// 返回 StatusCode 的 uint16 表示
	// Return the uint16 representation of StatusCode
	return uint16(c)
}

// Bytes 将 StatusCode 转换为字节切片
// Bytes converts StatusCode to a byte slice
func (c StatusCode) Bytes() []byte {
	// 如果 StatusCode 为 0，返回空字节切片
	// If StatusCode is 0, return an empty byte slice
	if c == 0 {
		return []byte{}
	}
	// 返回包含 StatusCode 高字节和低字节的字节切片
	// Return a byte slice containing the high byte and low byte of StatusCode
	return []byte{uint8(c >> 8), uint8(c << 8 >> 8)}
}

// Error 返回 StatusCode 对应的错误字符串
// Error returns the error string corresponding to StatusCode
func (c StatusCode) Error() string {
	// 返回包含错误信息的字符串
	// Return a string containing the error message
	return "gws: " + closeErrorMap[c]
}

// NewError 创建一个新的 Error 实例
// NewError creates a new Error instance
func NewError(code StatusCode, err error) *Error {
	// 返回包含指定状态码和错误的 Error 实例
	// Return an Error instance containing the specified status code and error
	return &Error{Code: code, Err: err}
}

// Error 结构体定义了一个包含错误和状态码的错误类型
// Error struct defines an error type containing an error and a status code
type Error struct {
	Err  error      // 错误信息
	Code StatusCode // 状态码
}

// Error 返回错误的字符串表示
// Error returns the string representation of the error
func (c *Error) Error() string {
	// 返回错误信息的字符串表示
	// Return the string representation of the error message
	return c.Err.Error()
}

// Errors 依次执行传入的函数，返回第一个遇到的错误
// Errors executes the passed functions in sequence and returns the first encountered error
func Errors(funcs ...func() error) error {
	// 遍历每个函数
	// Iterate over each function
	for _, f := range funcs {
		// 执行函数并检查是否有错误
		// Execute the function and check for an error
		if err := f(); err != nil {
			// 返回遇到的第一个错误
			// Return the first encountered error
			return err
		}
	}
	// 如果没有遇到错误，返回 nil
	// If no errors are encountered, return nil
	return nil
}
