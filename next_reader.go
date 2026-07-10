package gws

import (
	"bytes"
	"errors"
	"io"
	"net"

	"github.com/lxzan/gws/internal"
)

type nextMessageReader interface {
	io.Reader
	close() error
}

type uncompressedMessageReader struct {
	conn         *Conn
	framePayload int
	frameFIN     bool
	maskEnabled  bool
	maskKey      [4]byte
	maskOffset   int
	messageBytes int
	messageDone  bool
	streamDone   bool
}

type messageReader struct {
	conn          *Conn
	opcode        Opcode
	framePayload  int
	frameFIN      bool
	maskEnabled   bool
	maskKey       [4]byte
	maskOffset    int
	messageBytes  int
	messageDone   bool
	streamDone    bool
	compressedBuf *bytes.Buffer
	output        *bytes.Buffer
}

// NextReader
// 读取并返回下一条完整消息的 reader. messageType 对应 websocket opcode.
// 该方法不会触发 OnOpen/OnMessage 回调, 但会处理并派发控制帧事件.
// 如发生错误, 会触发错误事件并回收资源, 与 ReadLoop 结束时的处理逻辑一致.
//
// Reads and returns a reader for the next complete message. messageType is the websocket
// opcode. This method does not trigger OnOpen/OnMessage callbacks, but it does process and
// dispatch control-frame events. On error, it emits error/close events and reclaims resources,
// consistent with the handling at the end of ReadLoop.
func (c *Conn) NextReader() (messageType Opcode, r io.Reader, err error) {
	if err := c.closeNextReader(); err != nil {
		err = normalizeReadError(err)
		c.handleReadError(err)
		return 0, nil, err
	}

	h, payloadLen, readErr := c.readDataFrameHeader(false)
	if readErr != nil {
		readErr = normalizeReadError(readErr)
		c.handleReadError(readErr)
		return 0, nil, readErr
	}
	c.rr = c.resetNextMessageReader(h, payloadLen)
	return h.GetOpcode(), c.rr, nil
}

func (c *Conn) closeNextReader() error {
	if c.rr == nil {
		return nil
	}
	err := c.rr.close()
	c.rr = nil
	return err
}

func (c *Conn) resetNextMessageReader(h *frameHeader, payloadLen int) nextMessageReader {
	if !(c.pd.Enabled && h.GetRSV1()) {
		c.urr.reset(c, h, payloadLen)
		return &c.urr
	}

	c.crr.reset(c, h, payloadLen)
	return &c.crr
}

func (c *uncompressedMessageReader) reset(conn *Conn, h *frameHeader, payloadLen int) {
	*c = uncompressedMessageReader{
		conn:         conn,
		framePayload: payloadLen,
		frameFIN:     h.GetFIN(),
		maskEnabled:  h.GetMask(),
	}
	if c.maskEnabled {
		copy(c.maskKey[:], h.GetMaskKey())
	}
}

func (c *uncompressedMessageReader) Read(p []byte) (int, error) {
	if c.streamDone {
		return 0, io.EOF
	}

	n, err := c.read(p)
	if err != nil {
		if err == io.EOF {
			c.streamDone = true
			c.conn.rr = nil
			return n, err
		}
		return 0, c.failRead(err)
	}
	return n, nil
}

func (c *uncompressedMessageReader) close() error {
	if c.streamDone {
		return nil
	}
	defer func() {
		c.streamDone = true
	}()

	buf := binaryPool.Get(4096)
	defer binaryPool.Put(buf)
	p := buf.Bytes()
	p = p[:cap(p)]
	for {
		_, err := c.read(p)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (c *uncompressedMessageReader) read(p []byte) (int, error) {
	if c.messageDone {
		return 0, io.EOF
	}
	nTotal := 0
	for nTotal < len(p) {
		if c.framePayload == 0 {
			if c.frameFIN {
				c.messageDone = true
				return 0, io.EOF
			}
			h, payloadLen, err := c.conn.readDataFrameHeader(true)
			if err != nil {
				return nTotal, err
			}
			c.frameFIN = h.GetFIN()
			c.framePayload = payloadLen
			c.maskEnabled = h.GetMask()
			c.maskOffset = 0
			if c.maskEnabled {
				copy(c.maskKey[:], h.GetMaskKey())
			}
			if c.framePayload == 0 {
				continue
			}
		}

		want := len(p) - nTotal
		if want > c.framePayload {
			want = c.framePayload
		}
		dst := p[nTotal : nTotal+want]
		if err := internal.ReadN(c.conn.br, dst); err != nil {
			return nTotal, err
		}
		if c.maskEnabled {
			internal.MaskXOROffset(dst, c.maskKey[:], c.maskOffset)
			c.maskOffset += len(dst)
		}
		if err := c.trackPayload(len(dst)); err != nil {
			return nTotal, err
		}
		nTotal += len(dst)
		c.framePayload -= len(dst)
		if nTotal > 0 {
			return nTotal, nil
		}
	}
	return nTotal, nil
}

func (c *uncompressedMessageReader) trackPayload(n int) error {
	if n == 0 {
		return nil
	}
	c.messageBytes += n
	if c.messageBytes > c.conn.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}
	return nil
}

func (c *uncompressedMessageReader) failRead(err error) error {
	if err == nil {
		return nil
	}
	err = normalizeReadError(err)
	c.streamDone = true
	c.conn.rr = nil
	c.conn.handleReadError(err)
	return err
}

func (c *messageReader) reset(conn *Conn, h *frameHeader, payloadLen int) {
	c.releaseOutput()
	c.releaseCompressed()
	*c = messageReader{
		conn:         conn,
		opcode:       h.GetOpcode(),
		framePayload: payloadLen,
		frameFIN:     h.GetFIN(),
		maskEnabled:  h.GetMask(),
	}
	if c.maskEnabled {
		copy(c.maskKey[:], h.GetMaskKey())
	}
}

func (c *messageReader) Read(p []byte) (int, error) {
	if c.streamDone {
		return 0, io.EOF
	}

	if c.output == nil {
		if err := c.fillCompressedOutput(); err != nil {
			return 0, c.failRead(err)
		}
	}
	n, err := c.output.Read(p)
	if err == io.EOF {
		c.streamDone = true
		c.releaseOutput()
		c.conn.rr = nil
	}
	return n, err
}

func (c *messageReader) close() error {
	if c.streamDone {
		return nil
	}
	defer func() {
		c.streamDone = true
		c.releaseOutput()
		c.releaseCompressed()
	}()
	if c.output == nil {
		return c.fillCompressedOutput()
	}
	return nil
}

func (c *messageReader) fillCompressedOutput() error {
	if c.compressedBuf == nil {
		c.compressedBuf = binaryPool.Get(c.framePayload + len(flateTail))
	}
	var tmp *bytes.Buffer
	defer func() {
		if tmp != nil {
			binaryPool.Put(tmp)
		}
	}()

	for !c.messageDone {
		if c.framePayload == 0 {
			if c.frameFIN {
				c.messageDone = true
				break
			}
			h, payloadLen, err := c.conn.readDataFrameHeader(true)
			if err != nil {
				return err
			}
			c.frameFIN = h.GetFIN()
			c.framePayload = payloadLen
			c.maskEnabled = h.GetMask()
			c.maskOffset = 0
			if c.maskEnabled {
				copy(c.maskKey[:], h.GetMaskKey())
			}
			continue
		}
		if tmp == nil {
			tmp = binaryPool.Get(4096)
		}
		chunk := tmp.Bytes()
		if cap(chunk) > c.framePayload {
			chunk = chunk[:c.framePayload]
		} else {
			chunk = chunk[:cap(chunk)]
		}
		if err := internal.ReadN(c.conn.br, chunk); err != nil {
			return err
		}
		if c.maskEnabled {
			internal.MaskXOROffset(chunk, c.maskKey[:], c.maskOffset)
			c.maskOffset += len(chunk)
		}
		if err := c.trackPayload(chunk); err != nil {
			return err
		}
		c.compressedBuf = growNextReaderBuffer(c.compressedBuf, len(chunk))
		_, _ = c.compressedBuf.Write(chunk)
		c.framePayload -= len(chunk)
	}

	msg := &Message{Opcode: c.opcode, Data: c.compressedBuf, compressed: true}
	c.compressedBuf = nil
	if err := c.conn.decompressMessage(msg); err != nil {
		return err
	}
	c.output = msg.Data
	return nil
}

func (c *messageReader) releaseCompressed() {
	if c.compressedBuf != nil {
		binaryPool.Put(c.compressedBuf)
		c.compressedBuf = nil
	}
}

func (c *messageReader) releaseOutput() {
	if c.output != nil {
		binaryPool.Put(c.output)
		c.output = nil
	}
}

func (c *messageReader) trackPayload(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	c.messageBytes += len(p)
	if c.messageBytes > c.conn.config.ReadMaxPayloadSize {
		return internal.CloseMessageTooLarge
	}
	return nil
}

func (c *messageReader) failRead(err error) error {
	if err == nil {
		return nil
	}
	err = normalizeReadError(err)
	c.streamDone = true
	c.releaseOutput()
	c.releaseCompressed()
	c.conn.rr = nil
	c.conn.handleReadError(err)
	return err
}

func normalizeReadError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case internal.StatusCode, *internal.Error, *CloseError:
		return err
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return internal.CloseAbnormalClosure
	}
	return internal.NewError(internal.CloseInternalErr, err)
}

func growNextReaderBuffer(buf *bytes.Buffer, n int) *bytes.Buffer {
	if n <= 0 || buf.Cap() >= buf.Len()+n+len(flateTail) {
		return buf
	}
	next := binaryPool.Get(buf.Len() + n + len(flateTail))
	_, _ = next.Write(buf.Bytes())
	binaryPool.Put(buf)
	return next
}
