package websocket

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/lxzan/gws/internal"
	"io"
)

func (c *Conn) readControl() (continued bool, retErr error) {
	// RFC6455: All frames sent from client to server have this bit set to 1.
	if c.side == serverSide && !c.fh.GetMask() {
		return false, CloseProtocolError
	}

	//RFC6455:  Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return false, CloseProtocolError
	}

	var n = c.fh.GetLengthCode()
	// RFC6455: All control frames MUST have a payload length of 125 bytes or less and MUST NOT be fragmented.
	if n > internal.PayloadSizeLv1 {
		return false, CloseProtocolError
	}

	var payload = c.controlBuffer[:n]
	if err := c.readN(payload, int(n)); err != nil {
		return false, err
	}

	var maskOn = c.fh.GetMask()
	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return false, err
		}
		maskXOR(payload, c.fh[10:14])
	}

	switch c.fh.GetOpcode() {
	case Opcode_Ping:
		c.handler.OnPing(c, payload)
		return true, nil
	case Opcode_Pong:
		c.handler.OnPong(c, payload)
		return true, nil
	case Opcode_CloseConnection:
		switch n {
		case 0:
			c.emitClose(CloseNormalClosure, nil)
		case 1:
			c.emitClose(CloseProtocolError, nil)
		default:
			c.emitClose(Code(binary.BigEndian.Uint16(payload[:2])), payload[2:])
		}
		return false, nil
	default:
		return false, CloseUnsupportedData
	}
}

func (c *Conn) emitClose(code Code, reason []byte) {
	var msg = fmt.Sprintf("received close frame, code=%d, reason=%s", code.Uint16(), string(reason))
	c.debugLog(errors.New(msg))
	c.handler.OnClose(c, code, reason)
}

func (c *Conn) readMessage() (continued bool, retErr error) {
	if err := c.readN(c.fh[:2], 2); err != nil {
		return false, err
	}

	// read control frame
	var opcode = c.fh.GetOpcode()
	if !isDataFrame(opcode) {
		return c.readControl()
	}

	if opcode == Opcode_Text || opcode == Opcode_Binary {
		c.opcode = opcode
	}

	var fin = c.fh.GetFIN()
	var maskOn = c.fh.GetMask()
	var lengthCode = c.fh.GetLengthCode()
	var contentLength = int(lengthCode)

	// read data frame
	var buf *bytes.Buffer
	switch lengthCode {
	case 126:
		if err := c.readN(c.fh[2:4], 2); err != nil {
			return false, err
		}
		contentLength = int(binary.BigEndian.Uint16(c.fh[2:4]))
		buf = _pool.Get(contentLength)
	case 127:
		err := c.readN(c.fh[2:10], 8)
		if err != nil {
			return false, err
		}
		contentLength = int(binary.BigEndian.Uint64(c.fh[2:10]))
		buf = _pool.Get(contentLength)
	default:
		buf = _pool.Get(int(lengthCode))
	}

	if contentLength > _config.MaxContentLength {
		return false, CloseMessageTooLarge
	}

	// RFC6455: All frames sent from client to server have this bit set to 1.
	if c.side == serverSide && !maskOn {
		return false, CloseProtocolError
	}

	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return false, err
		}
		if _, err := io.CopyN(buf, c.netConn, int64(contentLength)); err != nil {
			return false, err
		}
		maskXOR(buf.Bytes(), c.fh[10:14])
	} else {
		if _, err := io.CopyN(buf, c.netConn, int64(contentLength)); err != nil {
			return false, err
		}
	}

	if !fin || (fin && opcode == Opcode_Continuation) {
		if err := writeN(c.fragmentBuffer, buf.Bytes(), contentLength); err != nil {
			return false, err
		}
		if c.fragmentBuffer.Len() > _config.MaxContentLength {
			return false, CloseMessageTooLarge
		}
	}

	if fin {
		switch opcode {
		case Opcode_Continuation:
			if err := writeN(buf, c.fragmentBuffer.Bytes(), contentLength); err != nil {
				return false, err
			}
			c.mq.Push(&Message{compressed: c.compress, opcode: c.opcode, data: buf, index: -1})
			c.messageLoop()

			c.fragmentBuffer.Reset()
			if c.fragmentBuffer.Cap() > _config.ReadBufferSize {
				c.fragmentBuffer = bytes.NewBuffer(nil)
			}
		case Opcode_Text, Opcode_Binary:
			c.mq.Push(&Message{compressed: c.compress, opcode: opcode, data: buf, index: -1})
			c.messageLoop()
		default:
		}
	}
	return true, nil
}

func (c *Conn) messageLoop() {
	var ele = c.mq.Pop()
	if ele == nil {
		return
	}

	var d = ele.Data.(*Message)
	go func(msg *Message) {
		defer func() {
			exception := recover()
			if s, ok := exception.(string); ok && s == SIGNAL_ABORT {
				return
			}
			c.handler.OnRecover(c, exception)
		}()

		c.emitMessage(msg)
		c.mq.Done()
		c.messageLoop()
	}(d)
}

func (c *Conn) emitMessage(msg *Message) {
	if !msg.compressed {
		msg.Next(c)
		return
	}

	decompressor := c.decompressors.Select()
	plainText, err := decompressor.Decompress(msg.data)
	if err != nil {
		c.emitError(err)
		return
	}
	msg.data = plainText
	msg.Next(c)
	return
}
