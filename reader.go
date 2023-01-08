package gws

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/lxzan/gws/internal"
	"io"
	"sync/atomic"
)

func (c *Conn) readN(data []byte, n int) error {
	if n == 0 {
		return nil
	}
	num, err := io.ReadFull(c.rbuf, data)
	if err != nil {
		return err
	}
	if num != n {
		return internal.CloseNormalClosure
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
	if n > internal.Lv1 {
		return internal.CloseProtocolError
	}

	var buf = internal.NewBufferWithCap(n)
	if err := c.readN(c.fh[10:14], 4); err != nil {
		return err
	}
	if _, err := io.CopyN(buf, c.rbuf, int64(n)); err != nil {
		return err
	}
	maskXOR(buf.Bytes(), c.fh[10:14])

	var opcode = c.fh.GetOpcode()
	switch opcode {
	case OpcodePing:
		c.handler.OnPing(c, buf.Bytes())
		return nil
	case OpcodePong:
		c.handler.OnPong(c, buf.Bytes())
		return nil
	case OpcodeCloseConnection:
		return c.emitClose(buf)
	default:
		var err = errors.New(fmt.Sprintf("unexpected opcode: %d", opcode))
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

func (c *Conn) readMessage() error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return internal.CloseNormalClosure
	}
	if err := c.readN(c.fh[:2], 2); err != nil {
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
	var lengthCode = c.fh.GetLengthCode()
	var contentLength = int(lengthCode)

	// read data frame
	var buf *internal.Buffer
	switch lengthCode {
	case 126:
		if err := c.readN(c.fh[2:4], 2); err != nil {
			return err
		}
		contentLength = int(binary.BigEndian.Uint16(c.fh[2:4]))
		buf = _pool.Get(contentLength)
	case 127:
		err := c.readN(c.fh[2:10], 8)
		if err != nil {
			return err
		}
		contentLength = int(binary.BigEndian.Uint64(c.fh[2:10]))
		buf = _pool.Get(contentLength)
	default:
		buf = _pool.Get(int(lengthCode))
	}

	if contentLength > c.config.MaxContentLength {
		return internal.CloseMessageTooLarge
	}

	if err := c.readN(c.fh[10:14], 4); err != nil {
		return err
	}
	if _, err := io.CopyN(buf, c.rbuf, int64(contentLength)); err != nil {
		return err
	}
	maskXOR(buf.Bytes(), c.fh[10:14])

	if !fin && (opcode == OpcodeText || opcode == OpcodeBinary) {
		c.continuationCompressed = compressed
		c.continuationOpcode = opcode
		c.continuationBuffer = internal.NewBuffer(make([]byte, 0, contentLength))
	}

	if !fin || (fin && opcode == OpcodeContinuation) {
		if c.continuationBuffer == nil {
			return internal.CloseProtocolError
		}
		if err := writeN(c.continuationBuffer, buf.Bytes(), contentLength); err != nil {
			return err
		}
		if c.continuationBuffer.Len() > c.config.MaxContentLength {
			return internal.CloseMessageTooLarge
		}
		if !fin {
			return nil
		}
	}

	// Send unfragmented Text Message after Continuation Frame with FIN = false
	if c.continuationBuffer != nil && opcode != OpcodeContinuation {
		return internal.CloseProtocolError
	}
	switch opcode {
	case OpcodeContinuation:
		compressed = c.continuationCompressed
		msg := &Message{opcode: c.continuationOpcode, buf: c.continuationBuffer}
		c.continuationCompressed = false
		c.continuationOpcode = 0
		c.continuationBuffer = nil
		return c.emitMessage(msg, compressed)
	case OpcodeText, OpcodeBinary:
		return c.emitMessage(&Message{opcode: opcode, buf: buf}, compressed)
	default:
		var err = errors.New(fmt.Sprintf("unexpected opcode: %d", opcode))
		return internal.NewError(internal.CloseProtocolError, err)
	}
}

func (c *Conn) emitMessage(msg *Message, compressed bool) error {
	if compressed {
		data, err := c.decompressor.Decompress(msg.buf)
		if err != nil {
			return internal.NewError(internal.CloseInternalServerErr, err)
		}
		msg.buf = data
	}
	if c.config.CheckTextEncoding && !isTextValid(msg.opcode, msg.buf) {
		return internal.NewError(internal.CloseUnsupportedData, internal.ErrTextEncoding)
	}
	c.handler.OnMessage(c, msg)
	return nil
}
