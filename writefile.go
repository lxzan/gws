package gws

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

const segmentSize = 128 * 1024

// 获取大文件压缩器
// Get bigDeflater
func (c *Conn) getBigDeflater() *bigDeflater {
	if c.isServer {
		return c.config.bdPool.Get()
	}
	return (*bigDeflater)(c.deflater.cpsWriter)
}

// 回收大文件压缩器
// Recycle bigDeflater
func (c *Conn) putBigDeflater(d *bigDeflater) {
	if c.isServer {
		c.config.bdPool.Put(d)
	}
}

// 拆分io.Reader为小切片
// Split io.Reader into small slices
func (c *Conn) splitReader(r io.Reader, f func(index int, eof bool, p []byte) error) error {
	var buf = binaryPool.Get(segmentSize)
	defer binaryPool.Put(buf)

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

// WriteFile 大文件写入
// 采用分段写入技术, 减少写入过程中的内存占用
// Segmented write technology to reduce memory usage during write process
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
		frame, err := c.genFrame(opcode, internal.Bytes(p), frameConfig{
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
	} else {
		return c.splitReader(payload, cb)
	}
}

// 大文件压缩器
type bigDeflater flate.Writer

// 创建大文件压缩器
// Create a bigDeflater
func newBigDeflater(isServer bool, options PermessageDeflate) *bigDeflater {
	windowBits := internal.SelectValue(isServer, options.ServerMaxWindowBits, options.ClientMaxWindowBits)
	if windowBits == 15 {
		cpsWriter, _ := flate.NewWriter(nil, options.Level)
		return (*bigDeflater)(cpsWriter)
	} else {
		cpsWriter, _ := flate.NewWriterWindow(nil, internal.BinaryPow(windowBits))
		return (*bigDeflater)(cpsWriter)
	}
}

func (c *bigDeflater) FlateWriter() *flate.Writer { return (*flate.Writer)(c) }

// Compress 压缩
func (c *bigDeflater) Compress(r io.WriterTo, w *flateWriter, dict []byte) error {
	if err := compressTo(c.FlateWriter(), r, w, dict); err != nil {
		return err
	}
	return w.Flush()
}

// 写入代理
// 将切片透传给回调函数, 以实现分段写入功能
// Write proxy
// Passthrough slices to the callback function for segmented writes.
type flateWriter struct {
	index   int
	buffers []*bytes.Buffer
	cb      func(index int, eof bool, p []byte) error
}

// 是否可以执行回调函数
// Whether the callback function can be executed
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

// 聚合写入, 减少syscall.write调用次数
// Aggregate writes, reducing the number of syscall.write calls
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
		if tail := buf.Bytes()[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			buf.Truncate(n - 4)
		}
	}
	var err = c.cb(c.index, true, buf.Bytes())
	c.index++
	binaryPool.Put(buf)
	return err
}

// 将io.Reader包装为io.WriterTo
// Wrapping io.Reader as io.WriterTo
type readerWrapper struct {
	r  io.Reader
	sw *slideWindow
}

// WriteTo 写入内容, 并更新字典
// Write the contents, and update the dictionary
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
