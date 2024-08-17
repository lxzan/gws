package gws

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"io"
	"math"
)

const segmentSize = 128 * 1024

// 获取大文件压缩器
func (c *Conn) getBigDeflater() *bigDeflater {
	if c.isServer {
		return c.config.bdPool.Get()
	}
	return c.deflater.ToBigDeflater()
}

// 回收大文件压缩器
func (c *Conn) putBigDeflater(d *bigDeflater) {
	if c.isServer {
		c.config.bdPool.Put(d)
	}
}

// 拆分io.Reader为小切片
func (c *Conn) splitReader(r io.Reader, f func(index int, eof bool, p []byte) error) error {
	var buf = binaryPool.Get(segmentSize)
	var p = buf.Bytes()[:segmentSize]
	var n, index = 0, 0
	var err error
	for n, err = r.Read(p); err == nil || errors.Is(err, io.EOF); n, err = r.Read(p) {
		eof := errors.Is(err, io.EOF)
		if err = f(index, eof, p[:n]); err != nil {
			return err
		}
		index++
		if eof {
			break
		}
	}
	return err
}

// WriteReader 大文件写入
// 采用分段写入技术, 大大减少内存占用
func (c *Conn) WriteReader(opcode Opcode, payload io.Reader) error {
	err := c.doWriteReader(opcode, payload)
	c.emitError(err)
	return err
}

func (c *Conn) doWriteReader(opcode Opcode, payload io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cb = func(index int, eof bool, p []byte) error {
		op := internal.SelectValue(index == 0, opcode, OpcodeContinuation)
		frame, err := c.genFrame(op, eof, false, internal.Bytes(p), false)
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
		err := deflater.Compress(payload, fw, c.getCpsDict(false), &c.cpsWindow)
		c.putBigDeflater(deflater)
		return err
	} else {
		return c.splitReader(payload, cb)
	}
}

// 大文件压缩器
type bigDeflater struct {
	cpsWriter *flate.Writer
}

// 初始化大文件压缩器
// Initialize the bigDeflater
func (c *bigDeflater) initialize(isServer bool, options PermessageDeflate) *bigDeflater {
	windowBits := internal.SelectValue(isServer, options.ServerMaxWindowBits, options.ClientMaxWindowBits)
	if windowBits == 15 {
		c.cpsWriter, _ = flate.NewWriter(nil, options.Level)
	} else {
		c.cpsWriter, _ = flate.NewWriterWindow(nil, internal.BinaryPow(windowBits))
	}
	return c
}

// Compress 压缩
func (c *bigDeflater) Compress(src io.Reader, dst *flateWriter, dict []byte, sw *slideWindow) error {
	if err := compressTo(c.cpsWriter, &readerWrapper{r: src, sw: sw}, dst, dict); err != nil {
		return err
	}
	return dst.Flush()
}

// 写入代理
// 将切片透传给回调函数, 以实现分段写入功能
type flateWriter struct {
	index   int
	buffers []*bytes.Buffer
	cb      func(index int, eof bool, p []byte) error
}

// 是否可以执行回调函数
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

// 聚合写入, 减少syscall.write次数
func (c *flateWriter) write(p []byte) {
	if len(c.buffers) == 0 {
		var buf = binaryPool.Get(segmentSize)
		c.buffers = append(c.buffers, buf)
	}
	var n = len(c.buffers)
	var tail = c.buffers[n-1]
	if tail.Len()+len(p) >= segmentSize {
		var buf = binaryPool.Get(segmentSize)
		c.buffers = append(c.buffers, buf)
		tail = buf
	}
	tail.Write(p)
}

func (c *flateWriter) Write(p []byte) (n int, err error) {
	c.write(p)
	if c.shouldCall() {
		err = c.cb(c.index, false, c.buffers[0].Bytes())
		binaryPool.Put(c.buffers[0])
		c.buffers = c.buffers[1:]
		c.index++
	}
	return n, err
}

func (c *flateWriter) Flush() error {
	var buf = c.buffers[0]
	for i := 1; i < len(c.buffers); i++ {
		buf.Write(c.buffers[i].Bytes())
		binaryPool.Put(c.buffers[i])
	}
	if n := buf.Len(); n >= 4 {
		compressedContent := buf.Bytes()
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			buf.Truncate(n - 4)
		}
	}
	var err = c.cb(c.index, true, buf.Bytes())
	c.index++
	binaryPool.Put(buf)
	return err
}

// 将io.Reader包装为io.WriterTo
type readerWrapper struct {
	r  io.Reader
	sw *slideWindow
}

// WriteTo 写入内容, 并更新字典
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

// 压缩公共函数
func compressTo(cpsWriter *flate.Writer, r io.WriterTo, w io.Writer, dict []byte) error {
	cpsWriter.ResetDict(w, dict)
	if _, err := r.WriteTo(cpsWriter); err != nil {
		return err
	}
	return cpsWriter.Flush()
}
