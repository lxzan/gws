package gws

import (
	"encoding/binary"
	"errors"
	"github.com/lxzan/gws/internal"
	"io"
	"strconv"
	"time"
)

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
	var payload = c.controlBuffer[0:n]
	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return err
		}
	}
	if err := c.readN(payload, int(n)); err != nil {
		return err
	}

	if maskOn {
		maskXOR(payload, c.fh[10:14])
	}

	switch c.fh.GetOpcode() {
	case OpcodePing:
		if c.pingTime.IsZero() || time.Since(c.pingTime) < c.configs.MinPingInterval {
			c.pingCount++
		} else {
			c.pingTime = time.Now()
			c.pingCount = 0
		}
		return c.emitMessage(&Message{opcode: OpcodePing, dbuf: internal.NewBuffer(payload)})
	case OpcodePong:
		return c.emitMessage(&Message{opcode: OpcodePong, dbuf: internal.NewBuffer(payload)})
	case OpcodeCloseConnection:
		switch n {
		case 0:
			return CloseNormalClosure
		case 1:
			return CloseProtocolError
		default:
			return CloseCode(binary.BigEndian.Uint16(payload[:2]))
		}
	default:
		return CloseUnsupportedData
	}
}

func (c *Conn) readMessage() error {
	if err := c.readN(c.fh[:2], 2); err != nil {
		return err
	}

	if err := c.netConn.SetReadDeadline(time.Now().Add(c.configs.ReadTimeout)); err != nil {
		return err
	}

	// read control frame
	var opcode = c.fh.GetOpcode()
	if !isDataFrame(opcode) {
		return c.readControl()
	}

	var fin = c.fh.GetFIN()
	var maskOn = c.fh.GetMask()
	var lengthCode = c.fh.GetLengthCode()
	var rsv1 = c.fh.GetRSV1()
	var contentLength = int(lengthCode)

	// RSV1, RSV2, RSV3:  1 bit each
	//
	//      MUST be 0 unless an extension is negotiated that defines meanings
	//      for non-zero values.  If a nonzero value is received and none of
	//      the negotiated extensions defines the meaning of such a nonzero
	//      value, the receiving endpoint MUST _Fail the WebSocket
	//      Connection_.
	if rsv1 != c.compressEnabled {
		return CloseProtocolError
	}

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
			c.continuationBuffer = internal.NewBuffer(nil)
		}
		if err := writeN(c.continuationBuffer, buf.Bytes(), contentLength); err != nil {
			return err
		}
		if c.continuationBuffer.Len() > c.configs.MaxContentLength {
			return CloseMessageTooLarge
		}
		if fin {
			msg := &Message{opcode: c.continuationOpcode, dbuf: c.continuationBuffer}
			c.continuationOpcode = 0
			c.continuationBuffer = nil
			return c.emitMessage(msg)
		}
	}

	switch opcode {
	case OpcodeText, OpcodeBinary:
		return c.emitMessage(&Message{opcode: opcode, dbuf: buf})
	default:
		return errors.New("unexpected opcode: " + strconv.Itoa(int(opcode)))
	}
}

func (c *Conn) emitMessage(msg *Message) error {
	if c.isCanceled() {
		return nil
	}

	if msg.opcode == OpcodePing || msg.opcode == OpcodePong {
		c.messageChan <- msg
		if c.pingCount >= 10 {
			err := errors.New("ping operation is too frequently")
			c.messageChan <- &Message{err: err}
		}
		return nil
	}
	if !c.compressEnabled {
		c.messageChan <- msg
		return nil
	}

	if err := c.decompressor.Decompress(msg); err != nil {
		return CloseInternalServerErr
	}
	c.messageChan <- msg
	return nil
}
