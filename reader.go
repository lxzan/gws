package gws

import (
	"encoding/binary"
	"errors"
	"github.com/lxzan/gws/internal"
	"io"
	"strconv"
	"sync/atomic"
)

var _pool *internal.BufferPool

func init() {
	_pool = internal.NewBufferPool()
}

func (c *Conn) readN(data []byte, n int) error {
	if n == 0 {
		return nil
	}
	num, err := io.ReadFull(c.rbuf, data)
	if err != nil {
		return err
	}
	if num != n {
		return CloseNormalClosure
	}
	return nil
}

// read control frame
func (c *Conn) readControl() error {
	// RFC6455: All frames sent from client to server have this bit set to 1.
	if !c.fh.GetMask() {
		return CloseProtocolError
	}

	//RFC6455:  Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return CloseProtocolError
	}

	var n = c.fh.GetLengthCode()
	// RFC6455: All control frames MUST have a payload length of 125 bytes or fewer and MUST NOT be fragmented.
	if n > internal.Lv1 {
		return CloseProtocolError
	}

	var maskOn = c.fh.GetMask()
	var payload = _pool.Get(internal.Lv1)
	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return err
		}
	}
	if _, err := io.CopyN(payload, c.rbuf, int64(n)); err != nil {
		return err
	}

	if maskOn {
		maskXOR(payload.Bytes(), c.fh[10:14])
	}

	switch c.fh.GetOpcode() {
	case OpcodePing:
		return c.emitMessage(&Message{opcode: OpcodePing, buf: payload}, false)
	case OpcodePong:
		return c.emitMessage(&Message{opcode: OpcodePong, buf: payload}, false)
	case OpcodeCloseConnection:
		if n == 1 {
			return CloseProtocolError
		}
		return c.emitMessage(&Message{opcode: OpcodeCloseConnection, closeCode: CloseNormalClosure, buf: payload}, false)
	default:
		return CloseProtocolError
	}
}

func (c *Conn) readMessage() error {
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
		return CloseProtocolError
	}

	// read control frame
	var opcode = c.fh.GetOpcode()
	var compressed = c.compressEnabled && c.fh.GetRSV1()
	if !opcode.IsDataFrame() {
		return c.readControl()
	}

	var fin = c.fh.GetFIN()
	var maskOn = c.fh.GetMask()
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

	if contentLength > c.configs.MaxContentLength {
		return CloseMessageTooLarge
	}

	// RFC6455: All frames sent from client to server have this bit set to 1.
	if !maskOn {
		return CloseProtocolError
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
			return CloseProtocolError
		}
		if err := writeN(c.continuationBuffer, buf.Bytes(), contentLength); err != nil {
			return err
		}
		if c.continuationBuffer.Len() > c.configs.MaxContentLength {
			return CloseMessageTooLarge
		}
		if !fin {
			return nil
		}
	}

	// Send unfragmented Text Message after Continuation Frame with FIN = false
	if c.continuationBuffer != nil && opcode != OpcodeContinuation {
		return CloseProtocolError
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
		return errors.New("unexpected opcode: " + strconv.Itoa(int(opcode)))
	}
}

func (c *Conn) emitMessage(msg *Message, compressed bool) error {
	if atomic.LoadUint32(&c.closed) == 1 {
		return CloseNormalClosure
	}
	if c.isCanceled() {
		return CloseServiceRestart
	}

	if compressed {
		data, err := c.decompressor.Decompress(msg.buf)
		if err != nil {
			return CloseInternalServerErr
		}
		msg.buf = data
	}

	if msg.opcode == OpcodeCloseConnection && msg.buf.Len() >= 2 {
		var b = make([]byte, 2, 2)
		_, _ = msg.buf.Read(b)
		msg.closeCode = CloseCode(binary.BigEndian.Uint16(b))
	}

	if !payloadValid(msg.opcode, msg.buf) {
		return CloseUnsupportedData
	}

	switch msg.opcode {
	case OpcodePing:
		c.handler.OnPing(c, msg)
	case OpcodePong:
		c.handler.OnPong(c, msg)
	case OpcodeText, OpcodeBinary:
		c.handler.OnMessage(c, msg)
	case OpcodeCloseConnection:
		if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
			code := msg.Code()
			c.handlerError(msg.Code(), msg.buf)
			c.handler.OnClose(c, msg)
			return code
		}
	}
	return nil
}
