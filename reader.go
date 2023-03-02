package gws

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/lxzan/gws/internal"
	"sync/atomic"
)

var _bpool = internal.NewBufferPool()

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
	var buf bytes.Buffer
	if n > 0 {
		if err := internal.CopyN(&buf, c.rbuf, int64(n)); err != nil {
			return err
		}
		internal.MaskXOR(buf.Bytes(), c.fh.GetMaskKey())
	}

	var opcode = c.fh.GetOpcode()
	switch opcode {
	case OpcodePing:
		c.handler.OnPing(c, buf.Bytes())
		return nil
	case OpcodePong:
		c.handler.OnPong(c, buf.Bytes())
		return nil
	case OpcodeCloseConnection:
		return c.emitClose(&buf)
	default:
		var err = errors.New(fmt.Sprintf("unexpected opcode: %d", opcode))
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

func (c *Conn) readMessage() error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return internal.CloseNormalClosure
	}

	contentLength, err := c.fh.Parse(c.rbuf)
	if err != nil {
		return err
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

	// RFC6455: All frames sent from client to server have this bit set to 1.
	if !c.fh.GetMask() {
		return internal.CloseProtocolError
	}

	// read control frame
	var opcode = c.fh.GetOpcode()
	var compressed = c.compressEnabled && c.fh.GetRSV1()
	if !opcode.IsDataFrame() {
		return c.readControl()
	}

	var fin = c.fh.GetFIN()
	var buf = _bpool.Get(contentLength)
	if contentLength > c.config.MaxContentLength {
		return internal.CloseMessageTooLarge
	}

	if err := internal.CopyN(internal.Buffer{Buffer: buf}, c.rbuf, int64(contentLength)); err != nil {
		return err
	}
	internal.MaskXOR(buf.Bytes(), c.fh.GetMaskKey())

	if !fin && (opcode == OpcodeText || opcode == OpcodeBinary) {
		c.continuationFrame.initialized = true
		c.continuationFrame.compressed = compressed
		c.continuationFrame.opcode = opcode
		c.continuationFrame.buffer = _bpool.Get(contentLength)
	}

	if !fin || (fin && opcode == OpcodeContinuation) {
		if !c.continuationFrame.initialized {
			return internal.CloseProtocolError
		}
		if err := internal.WriteN(c.continuationFrame.buffer, buf.Bytes(), buf.Len()); err != nil {
			return err
		}
		if c.continuationFrame.buffer.Len() > c.config.MaxContentLength {
			return internal.CloseMessageTooLarge
		}
		if !fin {
			return nil
		}
	}

	// Send unfragmented Text Message after Continuation Frame with FIN = false
	if c.continuationFrame.initialized && opcode != OpcodeContinuation {
		return internal.CloseProtocolError
	}
	switch opcode {
	case OpcodeContinuation:
		msg := &Message{Opcode: c.continuationFrame.opcode, Data: c.continuationFrame.buffer}
		myerr := c.emitMessage(msg, c.continuationFrame.compressed)
		c.continuationFrame.reset()
		return myerr
	case OpcodeText, OpcodeBinary:
		return c.emitMessage(&Message{Opcode: opcode, Data: buf}, compressed)
	}
	return internal.CloseNormalClosure
}

func (c *Conn) emitMessage(msg *Message, compressed bool) error {
	if compressed {
		data, err := c.decompressor.Decompress(msg.Data)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		msg.Data = data
	}
	if c.config.CheckTextEncoding && !isTextValid(msg.Opcode, msg.Data.Bytes()) {
		return internal.NewError(internal.CloseUnsupportedData, internal.ErrTextEncoding)
	}

	if c.config.AsyncReadEnabled {
		return c.readTQ.Push(func() { c.handler.OnMessage(c, msg) })
	} else {
		c.handler.OnMessage(c, msg)
	}

	return nil
}
