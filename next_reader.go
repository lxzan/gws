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

// nextReaderProxy 包装复用的底层 reader, 通过代数检测过期句柄
type nextReaderProxy struct {
	conn *Conn
	gen  uint32
	r    nextMessageReader
}

func (p *nextReaderProxy) Read(b []byte) (int, error) {
	if p.gen != p.conn.readGen {
		return 0, ErrConnClosed
	}
	return p.r.Read(b)
}

type frameCursor struct {
	framePayload int
	frameFIN     bool
	maskEnabled  bool
	maskKey      [4]byte
	maskOffset   int
	messageBytes int
	messageDone  bool
}

type uncompressedMessageReader struct {
	conn *Conn
	frameCursor
	streamDone bool
}

type messageReader struct {
	conn   *Conn
	opcode Opcode
	frameCursor
	streamDone    bool
	compressedBuf *bytes.Buffer
	output        *bytes.Buffer
}

func (c *frameCursor) reset(h *frameHeader, payloadLen int) {
	c.framePayload = payloadLen
	c.frameFIN = h.GetFIN()
	c.maskEnabled = h.GetMask()
	c.maskOffset = 0
	if c.maskEnabled {
		copy(c.maskKey[:], h.GetMaskKey())
	}
}

func (c *frameCursor) advance(conn *Conn) (bool, error) {
	if c.frameFIN {
		c.messageDone = true
		return true, nil
	}
	h, payloadLen, err := conn.readDataFrameHeader(true)
	if err != nil {
		return false, err
	}
	c.reset(h, payloadLen)
	return false, nil
}

func (c *frameCursor) trackPayload(limit, n int) error {
	if n == 0 {
		return nil
	}
	c.messageBytes += n
	if c.messageBytes > limit {
		return internal.CloseMessageTooLarge
	}
	return nil
}

// NextReader 读取并返回下一条完整消息的 reader, messageType 对应 WebSocket opcode
// 该方法不会触发 OnOpen/OnMessage 回调, 但会处理并派发控制帧事件
// 发生错误时会触发错误事件并回收资源, 与 ReadLoop 结束时的处理逻辑一致
func (c *Conn) NextReader() (messageType Opcode, r io.Reader, err error) {
	if c.isClosed() {
		return 0, nil, ErrConnClosed
	}
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
	c.readGen++
	gen := c.readGen
	c.rr = c.resetNextMessageReader(h, payloadLen)
	return h.GetOpcode(), &nextReaderProxy{conn: c, gen: gen, r: c.rr}, nil
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
		if c.urr == nil {
			c.urr = new(uncompressedMessageReader)
		}
		c.urr.reset(c, h, payloadLen)
		return c.urr
	}

	if c.crr == nil {
		c.crr = new(messageReader)
	}
	c.crr.reset(c, h, payloadLen)
	return c.crr
}

func (c *uncompressedMessageReader) reset(conn *Conn, h *frameHeader, payloadLen int) {
	*c = uncompressedMessageReader{conn: conn}
	c.frameCursor.reset(h, payloadLen)
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
			done, err := c.advance(c.conn)
			if err != nil {
				return nTotal, err
			}
			if done {
				return 0, io.EOF
			}
			if c.framePayload == 0 {
				continue
			}
		}

		want := len(p) - nTotal
		want = min(want, c.framePayload)
		dst := p[nTotal : nTotal+want]
		if err := internal.ReadN(c.conn.br, dst); err != nil {
			return nTotal, err
		}
		if c.maskEnabled {
			internal.MaskXOROffset(dst, c.maskKey[:], c.maskOffset)
			c.maskOffset += len(dst)
		}
		if err := c.trackPayload(c.conn.config.ReadMaxPayloadSize, len(dst)); err != nil {
			return nTotal, err
		}
		nTotal += len(dst)
		c.framePayload -= len(dst)
		return nTotal, nil
	}
	return nTotal, nil
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
	*c = messageReader{conn: conn, opcode: h.GetOpcode()}
	c.frameCursor.reset(h, payloadLen)
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
		if c.conn.dpsWindow.enabled {
			return c.fillCompressedOutput()
		}
		return c.drainCompressedInput()
	}
	return nil
}

func (c *messageReader) drainCompressedInput() error {
	buf := binaryPool.Get(4096)
	defer binaryPool.Put(buf)
	p := buf.Bytes()
	p = p[:cap(p)]
	for !c.messageDone {
		if c.framePayload == 0 {
			done, err := c.advance(c.conn)
			if err != nil {
				return err
			}
			if done {
				break
			}
			continue
		}
		want := c.framePayload
		want = min(want, len(p))
		if err := internal.ReadN(c.conn.br, p[:want]); err != nil {
			return err
		}
		if err := c.trackPayload(c.conn.config.ReadMaxPayloadSize, want); err != nil {
			return err
		}
		c.framePayload -= want
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
			done, err := c.advance(c.conn)
			if err != nil {
				return err
			}
			if done {
				break
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
		if err := c.trackPayload(c.conn.config.ReadMaxPayloadSize, len(chunk)); err != nil {
			return err
		}
		c.compressedBuf = growNextReaderBuffer(c.compressedBuf, len(chunk))
		_, _ = c.compressedBuf.Write(chunk)
		c.framePayload -= len(chunk)
	}

	msg := messagePool.Get()
	msg.Opcode = c.opcode
	msg.Data = c.compressedBuf
	msg.compressed = true
	c.compressedBuf = nil
	err := c.conn.decompressMessage(msg)
	c.output = msg.Data
	msg.recycle()
	return err
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
