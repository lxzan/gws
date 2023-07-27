package gws

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/lxzan/gws/internal"
)

func (c *Conn) checkMask(enabled bool) error {
	// RFC6455: All frames sent from client to server have this bit set to 1.
	if (c.isServer && !enabled) || (!c.isServer && enabled) {
		return internal.CloseProtocolError
	}
	return nil
}

// read control frame
func (c *Conn) readControl() error {
	//RFC6455:  Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return internal.CloseProtocolError
	}

	// RFC6455: All control frames MUST have a payload length of 125 bytes or fewer and MUST NOT be fragmented.
	var n = c.fh.GetLengthCode()
	if n > internal.ThresholdV1 {
		return internal.CloseProtocolError
	}

	// 不回收小块buffer, 控制帧一般payload长度为0
	var payload []byte
	if n > 0 {
		payload = make([]byte, n)
		if err := internal.ReadN(c.rbuf, payload, int(n)); err != nil {
			return err
		}
		maskEnabled := c.fh.GetMask()
		if err := c.checkMask(maskEnabled); err != nil {
			return err
		}
		if maskEnabled {
			internal.MaskXOR(payload, c.fh.GetMaskKey())
		}
	}

	var opcode = c.fh.GetOpcode()
	switch opcode {
	case OpcodePing:
		c.handler.OnPing(c, payload)
		return nil
	case OpcodePong:
		c.handler.OnPong(c, payload)
		return nil
	case OpcodeCloseConnection:
		return c.emitClose(bytes.NewBuffer(payload))
	default:
		var err = errors.New(fmt.Sprintf("unexpected opcode: %d", opcode))
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

func (c *Conn) readMessage() error {
	if c.isClosed() {
		return internal.CloseNormalClosure
	}

	contentLength, err := c.fh.Parse(c.rbuf)
	if err != nil {
		return err
	}
	if contentLength > c.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}

	// RSV1, RSV2, RSV3:  1 bit each
	//
	//      MUST be 0 unless an extension is negotiated that defines meanings
	//      for non-zero values.  If a nonzero value is received and none of
	//      the negotiated extensions defines the meaning of such a nonzero
	//      value, the receiving endpoint MUST _Fail the WebSocket
	//      Connection_.
	if !c.compressEnabled && (c.fh.GetRSV1() || c.fh.GetRSV2() || c.fh.GetRSV3()) {
		return internal.CloseProtocolError
	}

	maskEnabled := c.fh.GetMask()
	if err := c.checkMask(maskEnabled); err != nil {
		return err
	}

	// read control frame
	var opcode = c.fh.GetOpcode()
	var compressed = c.compressEnabled && c.fh.GetRSV1()
	if !opcode.isDataFrame() {
		return c.readControl()
	}

	var fin = c.fh.GetFIN()
	var buf, index = myBufferPool.Get(contentLength)
	var p = buf.Bytes()
	p = p[:contentLength]
	if err := internal.ReadN(c.rbuf, p, contentLength); err != nil {
		return err
	}
	if maskEnabled {
		internal.MaskXOR(p, c.fh.GetMaskKey())
	}

	if opcode != OpcodeContinuation && c.continuationFrame.initialized {
		return internal.CloseProtocolError
	}

	if fin && opcode != OpcodeContinuation {
		return c.emitMessage(&Message{index: index, Opcode: opcode, Data: bytes.NewBuffer(p)}, compressed)
	}

	if !fin && opcode != OpcodeContinuation {
		c.continuationFrame.initialized = true
		c.continuationFrame.compressed = compressed
		c.continuationFrame.opcode = opcode
		c.continuationFrame.buffer = bytes.NewBuffer(make([]byte, 0, contentLength))
	}

	if !c.continuationFrame.initialized {
		return internal.CloseProtocolError
	}
	if err := internal.WriteN(c.continuationFrame.buffer, p, len(p)); err != nil {
		return err
	} else {
		myBufferPool.Put(buf, index)
	}
	if c.continuationFrame.buffer.Len() > c.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}
	if !fin {
		return nil
	}

	msg := &Message{Opcode: c.continuationFrame.opcode, Data: c.continuationFrame.buffer}
	e := c.emitMessage(msg, c.continuationFrame.compressed)
	c.continuationFrame.reset()
	return e
}

func (c *Conn) emitMessage(msg *Message, compressed bool) (err error) {
	if compressed {
		data, index := msg.Data, msg.index
		msg.Data, msg.index, err = c.config.decompressors.Select().Decompress(msg.Data)
		myBufferPool.Put(data, index)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
	}
	if !c.isTextValid(msg.Opcode, msg.Bytes()) {
		return internal.NewError(internal.CloseUnsupportedData, internal.ErrTextEncoding)
	}

	if c.config.ReadAsyncEnabled {
		c.readQueue.Push(func() { c.handler.OnMessage(c, msg) })
	} else {
		c.handler.OnMessage(c, msg)
	}
	return nil
}
