package gws

import (
	"encoding/binary"
	"errors"
	"github.com/lxzan/gws/internal"
	"io"
	"strconv"
	"time"
	"unicode/utf8"
)

var _pool *internal.BufferPool

func init() {
	_pool = internal.NewBufferPool()
}

func (c *Conn) ReadMessage() <-chan *Message {
	return c.messageChan
}

func (c *Conn) readN(data []byte, n int) error {
	num, err := io.ReadFull(c.rbuf, data)
	if err != nil {
		return err
	}
	if num != n {
		return CloseGoingAway
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
		return c.emitMessage(&Message{opcode: OpcodePing, dbuf: payload})
	case OpcodePong:
		return c.emitMessage(&Message{opcode: OpcodePong, dbuf: payload})
	case OpcodeCloseConnection:
		switch n {
		case 0:
			return CloseNormalClosure
		case 1:
			return CloseProtocolError
		default:
			var data = payload.Bytes()
			if !utf8.Valid(data[2:]) {
				return CloseUnsupportedData
			}
			return CloseCode(binary.BigEndian.Uint16(data[:2]))
		}
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

	if err := c.netConn.SetReadDeadline(time.Now().Add(c.configs.ReadTimeout)); err != nil {
		return err
	}

	// read control frame
	var opcode = c.fh.GetOpcode()
	var compressed = c.compressEnabled && c.fh.GetRSV1()
	if !isDataFrame(opcode) {
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

	if !fin || (fin && opcode == OpcodeContinuation) {
		if c.continuationBuffer == nil {
			c.continuationOpcode = opcode
			c.continuationBuffer = internal.NewBuffer(make([]byte, 0, contentLength))
		}
		if err := writeN(c.continuationBuffer, buf.Bytes(), contentLength); err != nil {
			return err
		}
		if c.continuationBuffer.Len() > c.configs.MaxContentLength {
			return CloseMessageTooLarge
		}
		if fin {
			msg := &Message{opcode: c.continuationOpcode, dbuf: c.continuationBuffer, compressed: compressed}
			c.continuationOpcode = 0
			c.continuationBuffer = nil
			return c.emitMessage(msg)
		}
		return nil
	}

	// Send unfragmented Text Message after Continuation Frame with FIN = false
	if fin && c.continuationBuffer != nil && opcode != OpcodeContinuation {
		return CloseProtocolError
	}

	switch opcode {
	case OpcodeText, OpcodeBinary:
		return c.emitMessage(&Message{opcode: opcode, dbuf: buf, compressed: compressed})
	default:
		return errors.New("unexpected opcode: " + strconv.Itoa(int(opcode)))
	}
}

func (c *Conn) emitMessage(msg *Message) error {
	if c.isCanceled() {
		return nil
	}

	if !c.compressEnabled || msg.opcode == OpcodePing || msg.opcode == OpcodePong {
		if msg.opcode == OpcodeText && !utf8.Valid(msg.Bytes()) {
			return CloseUnsupportedData
		}
		c.messageChan <- msg
		return nil
	}

	if err := c.decompressor.Decompress(msg); err != nil {
		return CloseInternalServerErr
	}
	if msg.opcode == OpcodeText && !utf8.Valid(msg.Bytes()) {
		return CloseUnsupportedData
	}
	c.messageChan <- msg
	return nil
}
