package gws

import (
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"time"
)

// read control frame
func (c *Conn) readControl() (continued bool, retErr error) {
	// RFC6455: All frames sent from client to server have this bit set to 1.
	if !c.fh.GetMask() {
		return false, CloseProtocolError
	}

	//RFC6455:  Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return false, CloseProtocolError
	}

	var n = c.fh.GetLengthCode()
	// RFC6455: All control frames MUST have a payload length of 125 bytes or fewer and MUST NOT be fragmented.
	if n > internal.Lv1 {
		return false, CloseProtocolError
	}

	var maskOn = c.fh.GetMask()
	var payload = c.controlBuffer[0:n]
	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return false, err
		}
	}
	if err := c.readN(payload, int(n)); err != nil {
		return false, err
	}

	if maskOn {
		maskXOR(payload, c.fh[10:14])
	}

	switch c.fh.GetOpcode() {
	case OpcodePing:
		c.handler.OnPing(c, payload)
		return true, nil
	case OpcodePong:
		c.handler.OnPong(c, payload)
		return true, nil
	case OpcodeCloseConnection:
		switch n {
		case 0:
			_ = c.Close(CloseNormalClosure, nil)
		case 1:
			_ = c.Close(CloseProtocolError, nil)
		default:
			_ = c.Close(Code(binary.BigEndian.Uint16(payload[:2])), payload[2:])
		}
		return false, nil
	default:
		return false, CloseUnsupportedData
	}
}

func (c *Conn) readMessage() (continued bool, retErr error) {
	if err := c.readN(c.fh[:2], 2); err != nil {
		return false, err
	}

	if err := c.netConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout)); err != nil {
		return false, err
	}
	defer func() {
		_ = c.netConn.SetReadDeadline(time.Time{})
	}()

	// read control frame
	var opcode = c.fh.GetOpcode()
	// just for continuation opcode
	if opcode == OpcodeText || opcode == OpcodeBinary {
		c.opcode = opcode
	}

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

	if contentLength > c.conf.MaxContentLength {
		return false, CloseMessageTooLarge
	}

	// RFC6455: All frames sent from client to server have this bit set to 1.
	if !maskOn {
		return false, CloseProtocolError
	}

	if err := c.readN(c.fh[10:14], 4); err != nil {
		return false, err
	}
	if _, err := io.CopyN(buf, c.rbuf, int64(contentLength)); err != nil {
		return false, err
	}
	maskXOR(buf.Bytes(), c.fh[10:14])

	if !fin || (fin && opcode == OpcodeContinuation) {
		if err := writeN(c.fragment, buf.Bytes(), contentLength); err != nil {
			return false, err
		}
		if c.fragment.Len() > c.conf.MaxContentLength {
			return false, CloseMessageTooLarge
		}
	}

	if fin {
		switch opcode {
		case OpcodeContinuation:
			buf.Reset()
			if _, err := io.CopyN(buf, c.fragment, int64(c.fragment.Len())); err != nil {
				return false, err
			}
			c.mq.Push(NewMessage(c.compressEnabled, opcode, buf))
			c.messageLoop()

			c.fragment.Reset()
			if c.fragment.Cap() > c.conf.ReadBufferSize {
				c.fragment = internal.NewBuffer(nil)
			}
		case OpcodeText, OpcodeBinary:
			c.mq.Push(NewMessage(c.compressEnabled, opcode, buf))
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
			c.handler.OnRecover(c, exception)
		}()

		// server is stopping
		if c.isCanceled() {
			c.mq.Done()
			return
		}

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
		c.debugLog(err)
		c.emitError(CloseInternalServerErr)
		return
	}
	msg.data = plainText
	msg.Next(c)
	return
}
