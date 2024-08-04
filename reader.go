package gws

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/lxzan/gws/internal"
)

// checkMask 检查掩码设置是否符合 RFC6455 协议。
// checkMask checks if the mask setting complies with the RFC6455 protocol.
func (c *Conn) checkMask(enabled bool) error {
	// RFC6455: 所有从客户端发送到服务器的帧都必须设置掩码位为 1。
	// RFC6455: All frames sent from client to server must have the mask bit set to 1.
	if (c.isServer && !enabled) || (!c.isServer && enabled) {
		// 如果服务器端未启用掩码或客户端启用了掩码，则返回协议错误。
		// Return a protocol error if the server has the mask disabled or the client has the mask enabled.
		return internal.CloseProtocolError
	}

	// 掩码设置正确，返回 nil 表示没有错误。
	// The mask setting is correct, return nil indicating no error.
	return nil
}

// readControl 读取控制帧
// readControl reads a control frame
func (c *Conn) readControl() error {
	// RFC6455: 控制帧本身不能被分片。
	// RFC6455: Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return internal.CloseProtocolError
	}

	// RFC6455: 所有控制帧的有效载荷长度必须为 125 字节或更少，并且不能被分片。
	// RFC6455: All control frames MUST have a payload length of 125 bytes or fewer and MUST NOT be fragmented.
	var n = c.fh.GetLengthCode()

	// 控制帧的有效载荷长度不能超过 125 字节
	// The payload length of the control frame cannot exceed 125 bytes
	if n > internal.ThresholdV1 {
		return internal.CloseProtocolError
	}

	// 不回收小块 buffer，控制帧一般 payload 长度为 0
	// Do not recycle small buffers, control frames generally have a payload length of 0
	var payload []byte

	// 如果有效载荷长度大于 0，则读取有效载荷数据
	// If the payload length is greater than 0, read the payload data
	if n > 0 {
		// 创建一个长度为 n 的 payload 切片
		// Create a payload slice with length n
		payload = make([]byte, n)

		// 读取 n 字节的数据到 payload 中
		// Read n bytes of data into the payload
		if err := internal.ReadN(c.br, payload); err != nil {
			return err
		}

		// 如果启用了掩码，则对 payload 进行掩码操作
		// If masking is enabled, apply the mask to the payload
		if maskEnabled := c.fh.GetMask(); maskEnabled {
			internal.MaskXOR(payload, c.fh.GetMaskKey())
		}
	}

	// 获取操作码
	// Get the opcode
	var opcode = c.fh.GetOpcode()

	// 根据操作码处理不同的控制帧
	// Handle different control frames based on the opcode
	switch opcode {
	// 处理 Ping 帧
	// Handle Ping frame
	case OpcodePing:
		c.handler.OnPing(c, payload)
		return nil

	// 处理 Pong 帧
	// Handle Pong frame
	case OpcodePong:
		c.handler.OnPong(c, payload)
		return nil

	// 处理关闭连接帧
	// Handle Close Connection frame
	case OpcodeCloseConnection:
		return c.emitClose(bytes.NewBuffer(payload))

	// 处理未知操作码
	// Handle unknown opcode
	default:
		var err = fmt.Errorf("gws: unexpected opcode %d", opcode)
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

// readMessage 读取消息
// readMessage reads a message
func (c *Conn) readMessage() error {
	// 解析帧头并获取内容长度
	// Parse the frame header and get the content length
	contentLength, err := c.fh.Parse(c.br)
	if err != nil {
		return err
	}

	// 检查内容长度是否超过配置的最大有效载荷大小
	// Check if the content length exceeds the configured maximum payload size
	if contentLength > c.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	// RSV1, RSV2, RSV3: 每个占 1 位
	// 必须为 0，除非协商的扩展定义了非零值的含义。
	// 如果接收到非零值且没有协商的扩展定义该非零值的含义，接收端点必须关闭 WebSocket 连接。
	// RSV1, RSV2, RSV3: 1 bit each
	// MUST be 0 unless an extension is negotiated that defines meanings for non-zero values.
	// If a nonzero value is received and none of the negotiated extensions defines the meaning of such a nonzero value,
	// the receiving endpoint MUST _Fail the WebSocket Connection_.
	if !c.pd.Enabled && (c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3()) {
		return internal.CloseProtocolError
	}

	// 获取掩码标志
	// Get the mask flag
	maskEnabled := c.fh.GetMask()

	// 检查掩码设置是否符合协议
	// Check if the mask setting complies with the protocol
	if err := c.checkMask(maskEnabled); err != nil {
		return err
	}

	// 读取控制帧
	// Read control frame
	var opcode = c.fh.GetOpcode()

	// 检查是否启用了压缩并且 RSV1 标志已设置
	// Check if compression is enabled and the RSV1 flag is set
	var compressed = c.pd.Enabled && c.fh.GetRSV1()

	// 如果操作码不是数据帧，则读取控制帧
	// If the opcode is not a data frame, read the control frame
	if !opcode.isDataFrame() {
		return c.readControl()
	}

	// 获取 FIN 标志
	// Get the FIN flag
	var fin = c.fh.GetFIN()

	// 从内存池中获取一个缓冲区
	// Get a buffer from the memory pool
	var buf = binaryPool.Get(contentLength + len(flateTail))

	// 将缓冲区切片到内容长度
	// Slice the buffer to the content length
	var p = buf.Bytes()[:contentLength]

	// 创建一个 Message 实例，并在函数退出时关闭缓冲区
	// Create a Message instance and close the buffer when the function exits
	var closer = Message{Data: buf}
	defer closer.Close()

	// 读取指定长度的数据到缓冲区
	// Read the specified length of data into the buffer
	if err := internal.ReadN(c.br, p); err != nil {
		return err
	}

	// 如果启用了掩码，对数据进行掩码操作
	// If masking is enabled, apply the mask to the data
	if maskEnabled {
		internal.MaskXOR(p, c.fh.GetMaskKey())
	}
	// 检查操作码是否不是继续帧并且 continuationFrame 已初始化
	// Check if the opcode is not a continuation frame and the continuationFrame is initialized
	if opcode != OpcodeContinuation && c.continuationFrame.initialized {
		// 如果是，则返回协议错误
		// If so, return a protocol error
		return internal.CloseProtocolError
	}

	// 如果是最后一帧并且操作码不是继续帧
	// If it is the final frame and the opcode is not a continuation frame
	if fin && opcode != OpcodeContinuation {
		// 将缓冲区转换为字节切片
		// Convert the buffer to a byte slice
		*(*[]byte)(unsafe.Pointer(buf)) = p

		// 如果未启用压缩，则将 closer.Data 置为 nil
		// If compression is not enabled, set closer.Data to nil
		if !compressed {
			closer.Data = nil
		}

		// 发出消息并返回
		// Emit the message and return
		return c.emitMessage(&Message{Opcode: opcode, Data: buf, compressed: compressed})
	}

	// 如果不是最后一帧并且操作码不是继续帧
	// If it is not the final frame and the opcode is not a continuation frame
	if !fin && opcode != OpcodeContinuation {
		// 初始化 continuationFrame
		// Initialize the continuationFrame
		c.continuationFrame.initialized = true

		// 设置 continuationFrame 的压缩标志
		// Set the compressed flag of the continuationFrame
		c.continuationFrame.compressed = compressed

		// 设置 continuationFrame 的操作码
		// Set the opcode of the continuationFrame
		c.continuationFrame.opcode = opcode

		// 初始化 continuationFrame 的缓冲区，容量为 contentLength
		// Initialize the buffer of the continuationFrame with a capacity of contentLength
		c.continuationFrame.buffer = bytes.NewBuffer(make([]byte, 0, contentLength))
	}

	// 如果 continuationFrame 未初始化
	// If the continuationFrame is not initialized
	if !c.continuationFrame.initialized {
		// 返回协议错误
		// Return a protocol error
		return internal.CloseProtocolError
	}

	// 将数据写入 continuationFrame 的缓冲区
	// Write data to the continuationFrame's buffer
	c.continuationFrame.buffer.Write(p)

	// 如果缓冲区长度超过最大有效载荷大小
	// If the buffer length exceeds the maximum payload size
	if c.continuationFrame.buffer.Len() > c.config.ReadMaxPayloadSize {
		// 返回消息过大错误
		// Return a message too large error
		return internal.CloseMessageTooLarge
	}

	// 如果不是最后一帧，返回 nil
	// If it is not the final frame, return nil
	if !fin {
		return nil
	}

	// 创建一个新的 Message 实例
	// Create a new Message instance
	msg := &Message{
		// 设置操作码为 continuationFrame 的操作码
		// Set the opcode to the opcode of the continuationFrame
		Opcode: c.continuationFrame.opcode,

		// 设置数据为 continuationFrame 的缓冲区
		// Set the data to the buffer of the continuationFrame
		Data: c.continuationFrame.buffer,

		// 设置压缩标志为 continuationFrame 的压缩标志
		// Set the compressed flag to the compressed flag of the continuationFrame
		compressed: c.continuationFrame.compressed,
	}

	// 重置 continuationFrame
	// Reset the continuationFrame
	c.continuationFrame.reset()

	// 发出消息并返回
	// Emit the message and return
	return c.emitMessage(msg)
}

// dispatch 分发消息给消息处理器
// dispatch dispatches the message to the message handler
func (c *Conn) dispatch(msg *Message) error {
	// 使用 defer 确保在函数退出时调用 Recovery 方法进行错误恢复
	// Use defer to ensure the Recovery method is called for error recovery when the function exits
	defer c.config.Recovery(c.config.Logger)

	// 调用消息处理器的 OnMessage 方法处理消息
	// Call the OnMessage method of the message handler to process the message
	c.handler.OnMessage(c, msg)

	// 返回 nil 表示没有错误
	// Return nil indicating no error
	return nil
}

// emitMessage 处理并发出消息
// emitMessage processes and emits the message
func (c *Conn) emitMessage(msg *Message) (err error) {
	// 如果消息是压缩的，先解压缩消息数据
	// If the message is compressed, decompress the message data first
	if msg.compressed {
		msg.Data, err = c.deflater.Decompress(msg.Data, c.getDpsDict())
		if err != nil {
			// 如果解压缩失败，返回内部服务器错误
			// If decompression fails, return an internal server error
			return internal.NewError(internal.CloseInternalServerErr, err)
		}

		// 将解压缩后的数据写入 dpsWindow
		// Write the decompressed data to dpsWindow
		c.dpsWindow.Write(msg.Bytes())
	}

	// 检查文本消息的编码是否有效
	// Check if the text message encoding is valid
	if !c.isTextValid(msg.Opcode, msg.Bytes()) {
		// 如果编码无效，返回不支持的数据错误
		// If the encoding is invalid, return an unsupported data error
		return internal.NewError(internal.CloseUnsupportedData, ErrTextEncoding)
	}

	// 如果启用了并行处理，则将消息放入读取队列并发处理
	// If parallel processing is enabled, put the message into the read queue for concurrent processing
	if c.config.ParallelEnabled {
		return c.readQueue.Go(msg, c.dispatch)
	}

	// 否则，直接分发消息
	// Otherwise, directly dispatch the message
	return c.dispatch(msg)
}
