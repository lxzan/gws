package websocket

import (
	"bytes"
	"encoding/binary"
	"github.com/lxzan/websocket/internal"
	"io"
)

func (c *Conn) readControl() error {
	//RFC6455:  Control frames themselves MUST NOT be fragmented.
	if !c.fh.GetFIN() {
		return CloseProtocolError
	}

	var n = c.fh.GetLengthCode()
	// RFC6455: All control frames MUST have a payload length of 125 bytes or less and MUST NOT be fragmented.
	if n > internal.PayloadSizeLv1 {
		return CloseProtocolError
	}

	var payload = make([]byte, n, n)
	if err := c.readN(payload, int(n)); err != nil {
		return err
	}

	var maskOn = c.fh.GetMask()
	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return err
		}
		maskXOR(payload, c.fh[10:14])
	}

	switch c.fh.GetOpcode() {
	case Opcode_Ping:
		c.handler.OnPing(c, payload)
		return nil
	case Opcode_Pong:
		c.handler.OnPong(c, payload)
		return nil
	case Opcode_CloseConnection:
		switch n {
		case 0:
			c.emitDisconnect(CloseNormalClosure, nil)
		case 1:
			c.emitDisconnect(CloseProtocolError, nil)
		default:
			c.emitDisconnect(Code(binary.BigEndian.Uint16(payload[:2])), payload[2:])
		}
		return ERR_DISCONNECT
	default:
		return CloseAbnormalClosure
	}
}

func (c *Conn) emitDisconnect(code Code, reason []byte) {
	c.handler.OnClose(c, code, reason)
}

func (c *Conn) readMessage() error {
	if err := c.readN(c.fh[:2], 2); err != nil {
		return err
	}

	var fin = c.fh.GetFIN()
	var maskOn = c.fh.GetMask()
	var opcode = c.fh.GetOpcode()
	var lengthCode = c.fh.GetLengthCode()
	var contentLength = int(lengthCode)

	// RFC6455: All frames sent from client to server have this bit set to 1.
	if c.side == serverSide && !maskOn {
		return CloseProtocolError
	}

	//if maskOn {
	//	err := c.readN(c.fh[10:14], 4)
	//	if err != nil {
	//		return err
	//	}
	//}

	// read control frame
	if !isDataFrame(opcode) {
		return c.readControl()
	}

	// read data frame
	var buf *bytes.Buffer
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
		//buf = _pool.Get(int(lengthCode))
		buf = bytes.NewBuffer(nil)
	}

	if contentLength > _config.MaxContentLength {
		return CloseMessageTooBig
	}

	if maskOn {
		if err := c.readN(c.fh[10:14], 4); err != nil {
			return err
		}
		if _, err := io.CopyN(buf, c.netConn, int64(contentLength)); err != nil {
			return err
		}
		maskXOR(buf.Bytes(), c.fh[10:14])
	} else {
		if _, err := io.CopyN(buf, c.netConn, int64(contentLength)); err != nil {
			return err
		}
	}

	if !fin || (fin && opcode == Opcode_Continuation) {
		if _, err := io.CopyN(c.fragmentBuffer, buf, int64(contentLength)); err != nil {
			return err
		}
		if c.fragmentBuffer.Len() > _config.MaxContentLength {
			return CloseMessageTooBig
		}
	}

	if fin {
		switch opcode {
		case Opcode_Continuation:
			if _, err := io.Copy(buf, c.fragmentBuffer); err != nil {
				return err
			}
			c.mq.Push(&Message{compressed: c.compress, opcode: opcode, data: buf, index: -1})
			c.messageLoop()
		case Opcode_Text, Opcode_Binary:
			c.mq.Push(&Message{compressed: c.compress, opcode: opcode, data: buf, index: -1})
			c.messageLoop()
		default:
		}

		c.fragmentBuffer.Reset()
		if c.fragmentBuffer.Cap() > _config.ReadBufferSize {
			c.fragmentBuffer = bytes.NewBuffer(nil)
		}
	}
	return nil
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
