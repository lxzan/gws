package gws

import (
	"bytes"
	"errors"
	"io"
	"net"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

const segmentSize = 128 * 1024

// getBigDeflater 获取大文件压缩器
func (c *Conn) getBigDeflater() *bigDeflater {
	if c.isServer {
		return c.config.bdPool.Get()
	}
	return (*bigDeflater)(c.deflater.cpsWriter)
}

// putBigDeflater 回收大文件压缩器
func (c *Conn) putBigDeflater(d *bigDeflater) {
	if c.isServer {
		c.config.bdPool.Put(d)
	}
}

// splitReader 拆分 io.Reader 为小切片
func (c *Conn) splitReader(r io.Reader, f func(index int, eof bool, p []byte) error) error {
	var buf = binaryPool.Get(segmentSize)
	defer binaryPool.Put(buf)

	var p = buf.Bytes()[:segmentSize]
	var index = 0
	for {
		n, err := io.ReadFull(r, p)
		switch {
		case err == nil:
			if err = f(index, false, p[:n]); err != nil {
				return err
			}
			index++
		case errors.Is(err, io.ErrUnexpectedEOF):
			return f(index, true, p[:n])
		case errors.Is(err, io.EOF):
			return f(index, true, p[:0])
		default:
			return err
		}
	}
}

// WriteFile 大文件写入, 采用分段写入减少内存占用
func (c *Conn) WriteFile(opcode Opcode, payload io.Reader) error {
	err := c.doWriteFile(opcode, payload)
	c.emitError(false, err)
	return err
}

func (c *Conn) doWriteFile(opcode Opcode, payload io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cb = func(index int, eof bool, p []byte) error {
		if index > 0 {
			opcode = OpcodeContinuation
		}
		frame, err := c.genFrameBytes(opcode, p, frameConfig{
			fin:           eof,
			compress:      false,
			broadcast:     false,
			checkEncoding: false,
		})
		if err != nil {
			return err
		}
		if c.pd.Enabled && index == 0 {
			frame.Bytes()[0] |= uint8(64)
		}
		if c.isClosed() {
			return ErrConnClosed
		}
		err = internal.WriteN(c.conn, frame.Bytes())
		binaryPool.Put(frame)
		return err
	}

	if c.pd.Enabled {
		var deflater = c.getBigDeflater()
		var fw = &flateWriter{cb: cb}
		var reader = &readerWrapper{r: payload, sw: &c.cpsWindow}
		err := deflater.Compress(reader, fw, c.cpsWindow.dict)
		c.putBigDeflater(deflater)
		return err
	}
	return c.splitReader(payload, func(index int, eof bool, p []byte) error {
		return c.writeFileFrame(opcode, index, eof, p)
	})
}

func (c *Conn) writeFileFrame(opcode Opcode, index int, eof bool, p []byte) error {
	if index > 0 {
		opcode = OpcodeContinuation
	}
	if len(p) > c.config.WriteMaxPayloadSize {
		return ErrMessageTooLarge
	}
	if c.isClosed() {
		return ErrConnClosed
	}

	header := frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, eof, false, opcode, len(p), c.nextMaskKey())
	if !c.isServer {
		internal.MaskXOR(p, maskBytes)
	}

	buffers := net.Buffers{header[:headerLength], p}
	n, err := buffers.WriteTo(c.conn)
	if err != nil {
		return err
	}
	if n != int64(headerLength+len(p)) {
		return io.ErrShortWrite
	}
	return nil
}

// bigDeflater 大文件压缩器
type bigDeflater flate.Writer

// newBigDeflater 创建大文件压缩器
func newBigDeflater(isServer bool, options PermessageDeflate) *bigDeflater {
	return (*bigDeflater)(newCpsWriter(isServer, options))
}

func (c *bigDeflater) FlateWriter() *flate.Writer { return (*flate.Writer)(c) }

// Compress 压缩
func (c *bigDeflater) Compress(r io.WriterTo, w *flateWriter, dict []byte) error {
	if err := compressTo(c.FlateWriter(), r, w, dict); err != nil {
		return err
	}
	return w.Flush()
}

// flateWriter 写入代理, 透传切片给回调实现分段写入
type flateWriter struct {
	index   int
	buffers []*bytes.Buffer
	cb      func(index int, eof bool, p []byte) error
}

// shouldCall 判断是否可执行回调函数
func (c *flateWriter) shouldCall() bool {
	var n = len(c.buffers)
	if n < 2 {
		return false
	}
	var sum = 0
	for i := 1; i < n; i++ {
		sum += c.buffers[i].Len()
	}
	return sum >= 4
}

// write 聚合写入, 减少 syscall.write 调用次数
func (c *flateWriter) write(p []byte) {
	var size = internal.Max(segmentSize, len(p))
	if len(c.buffers) == 0 {
		c.buffers = append(c.buffers, binaryPool.Get(size))
	}
	var n = len(c.buffers)
	var tail = c.buffers[n-1]
	if tail.Len()+len(p)+frameHeaderSize > tail.Cap() {
		tail = binaryPool.Get(size)
		c.buffers = append(c.buffers, tail)
	}
	tail.Write(p)
}

// Write 遵守 io.Writer 契约, 始终返回 len(p)
func (c *flateWriter) Write(p []byte) (n int, err error) {
	c.write(p)
	if c.shouldCall() {
		err = c.cb(c.index, false, c.buffers[0].Bytes())
		binaryPool.Put(c.buffers[0])
		c.buffers = c.buffers[1:]
		c.index++
	}
	return len(p), err
}

func (c *flateWriter) Flush() error {
	var buf = c.buffers[0]
	for i := 1; i < len(c.buffers); i++ {
		buf.Write(c.buffers[i].Bytes())
		binaryPool.Put(c.buffers[i])
	}
	stripSyncFlushTail(buf)
	var err = c.cb(c.index, true, buf.Bytes())
	c.index++
	binaryPool.Put(buf)
	return err
}

// readerWrapper 将 io.Reader 包装为 io.WriterTo
type readerWrapper struct {
	r  io.Reader
	sw *slideWindow
}

// WriteTo 写入内容并更新字典
func (c *readerWrapper) WriteTo(w io.Writer) (int64, error) {
	var buf = binaryPool.Get(segmentSize)
	defer binaryPool.Put(buf)

	var p = buf.Bytes()[:segmentSize]
	var sum, n = 0, 0
	var err error
	for n, err = c.r.Read(p); err == nil || errors.Is(err, io.EOF); n, err = c.r.Read(p) {
		eof := errors.Is(err, io.EOF)
		if _, err = w.Write(p[:n]); err != nil {
			return int64(sum), err
		}
		sum += n
		_, _ = c.sw.Write(p[:n])
		if eof {
			break
		}
	}
	return int64(sum), err
}
