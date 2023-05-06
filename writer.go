package gws

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/lxzan/gws/internal"
)

type (
	Codec interface {
		NewEncoder(io.Writer) Encoder
	}

	Encoder interface {
		Encode(v interface{}) error
	}

	jsonCodec struct{}
)

func (c jsonCodec) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

// WriteClose proactively close the connection
// code: https://developer.mozilla.org/zh-CN/docs/Web/API/CloseEvent#status_codes
// 通过emitError发送关闭帧, 将连接状态置为关闭, 用于服务端主动断开连接
// 没有特殊原因的话, 建议code=0, reason=nil
func (c *Conn) WriteClose(code uint16, reason []byte) {
	var err = internal.NewError(internal.StatusCode(code), internal.GwsError(""))
	if len(reason) > 0 {
		err.Err = errors.New(string(reason))
	}
	c.emitError(err)
}

// WritePing write ping frame
func (c *Conn) WritePing(payload []byte) error {
	return c.WriteMessage(OpcodePing, payload)
}

// WritePong write pong frame
func (c *Conn) WritePong(payload []byte) error {
	return c.WriteMessage(OpcodePong, payload)
}

// WriteString write text frame
// force convert string to []byte
func (c *Conn) WriteString(s string) error {
	return c.WriteMessage(OpcodeText, []byte(s))
}

// WriteMessage 发送消息
func (c *Conn) WriteMessage(opcode Opcode, payload []byte) error {
	if c.isClosed() {
		return internal.ErrConnClosed
	}
	err := c.doWrite(opcode, payload)
	c.emitError(err)
	return err
}

// 执行写入逻辑, 关闭状态置为1后还能写, 以便发送关闭帧
// Execute the write logic, and write after the close state is set to 1, so that the close frame can be sent
func (c *Conn) doWrite(opcode Opcode, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	if opcode == OpcodeText && !c.isTextValid(opcode, payload) {
		return internal.NewError(internal.CloseUnsupportedData, internal.ErrTextEncoding)
	}

	if c.compressEnabled && opcode.IsDataFrame() && len(payload) >= c.config.CompressThreshold {
		return c.compressAndWrite(opcode, payload)
	}

	var n = len(payload)
	if n > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, false, opcode, n)
	var totalSize = n + headerLength
	var buf = _bpool.Get(totalSize)
	buf.Write(header[:headerLength])
	buf.Write(payload)
	var contents = buf.Bytes()
	if !c.isServer {
		internal.MaskXOR(contents[headerLength:], maskBytes)
	}
	var err = internal.WriteN(c.conn, contents, totalSize)
	_bpool.Put(buf)
	return err
}

// WriteAsync 异步非阻塞地写入消息
// Write messages asynchronously and non-blockingly
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) error {
	if c.isClosed() {
		return internal.ErrConnClosed
	}
	return c.writeQueue.Push(func() { c.emitError(c.doWrite(opcode, payload)) })
}

// WriteAny 以特定编码写入数据
// 使用此方法时, CheckUtf8Enabled=false且CompressThreshold选项无效
// Write data in a specific encoding
// When using this method, CheckUtf8Enabled=false and CompressThreshold option is disabled
func (c *Conn) WriteAny(codec Codec, opcode Opcode, v interface{}) error {
	if c.isClosed() {
		return internal.ErrConnClosed
	}

	var buf = _bpool.Get(internal.Lv3)
	c.wmu.Lock()
	err := c.doWriteAny(opcode, v, codec, buf)
	c.wmu.Unlock()
	_bpool.Put(buf)

	c.emitError(err)
	return err
}

func (c *Conn) doWriteAny(opcode Opcode, v interface{}, codec Codec, buf *bytes.Buffer) error {
	buf.Write(_padding[0:])
	var compress = c.compressEnabled && opcode.IsDataFrame()
	var err error
	if compress {
		err = _cps.Select(c.config.CompressLevel).CompressAny(codec, v, buf)
	} else {
		err = codec.NewEncoder(buf).Encode(v)
	}
	if err != nil {
		return err
	}
	return c.leftTrimAndWrite(opcode, buf, compress)
}

func (c *Conn) compressAndWrite(opcode Opcode, payload []byte) error {
	var buf = _bpool.Get(len(payload) / 2)
	defer _bpool.Put(buf)
	buf.Write(_padding[0:])
	if err := _cps.Select(c.config.CompressLevel).Compress(payload, buf); err != nil {
		return err
	}
	return c.leftTrimAndWrite(opcode, buf, true)
}

func (c *Conn) leftTrimAndWrite(opcode Opcode, buf *bytes.Buffer, compress bool) error {
	var contents = buf.Bytes()
	var payloadSize = buf.Len() - frameHeaderSize
	if payloadSize > c.config.WriteMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, true, compress, opcode, payloadSize)
	if !c.isServer {
		internal.MaskXOR(contents[frameHeaderSize:], maskBytes)
	}
	contents = contents[frameHeaderSize-headerLength:]
	copy(contents[:headerLength], header[:headerLength])
	return internal.WriteN(c.conn, contents, payloadSize+headerLength)
}
