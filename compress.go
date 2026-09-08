package gws

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

// deflate压缩算法的尾部标记
var flateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

// flateFlushTailLen 压缩输出尾部同步刷新标记(0x00 0x00 0xff 0xff)的长度,
// RFC7692 要求发送压缩消息前移除该标记.
const flateFlushTailLen = 4

type deflaterPool struct {
	serial uint64
	num    uint64
	pool   []*deflater
}

// 初始化deflaterPool
func (c *deflaterPool) initialize(options PermessageDeflate, limit int) *deflaterPool {
	c.num = uint64(options.PoolSize)
	for i := uint64(0); i < c.num; i++ {
		c.pool = append(c.pool, new(deflater).initialize(true, options, limit))
	}
	return c
}

// Select 从deflaterPool中选择一个deflater对象
func (c *deflaterPool) Select() *deflater {
	var j = atomic.AddUint64(&c.serial, 1) & (c.num - 1)
	return c.pool[j]
}

type deflater struct {
	dpsLocker sync.Mutex
	limit     int
	dpsBuffer *bytes.Buffer
	dpsReader io.ReadCloser
	rs        flate.Resetter
	lr        limitedReader
	cpsLocker sync.Mutex
	cpsWriter *flate.Writer
}

// 创建压缩器, 按连接角色选择窗口位数
func newCpsWriter(isServer bool, options PermessageDeflate) *flate.Writer {
	windowBits := internal.SelectValue(isServer, options.ServerMaxWindowBits, options.ClientMaxWindowBits)
	if windowBits == 15 {
		cpsWriter, _ := flate.NewWriter(nil, options.Level)
		return cpsWriter
	}
	cpsWriter, _ := flate.NewWriterWindow(nil, internal.BinaryPow(windowBits))
	return cpsWriter
}

// 初始化deflater
func (c *deflater) initialize(isServer bool, options PermessageDeflate, limit int) *deflater {
	c.dpsReader = flate.NewReader(nil)
	c.rs = c.dpsReader.(flate.Resetter)
	c.dpsBuffer = bytes.NewBuffer(nil)
	c.limit = limit
	c.cpsWriter = newCpsWriter(isServer, options)
	return c
}

// 重置deflate reader
func (c *deflater) resetFR(r io.Reader, dict []byte) {
	_ = c.rs.Reset(r, dict) // 必返回空指针
	if c.dpsBuffer.Cap() > int(bufferThreshold) {
		c.dpsBuffer = bytes.NewBuffer(nil)
	}
	c.dpsBuffer.Reset()
}

// Decompress 解压
func (c *deflater) Decompress(src *bytes.Buffer, dict []byte) (*bytes.Buffer, error) {
	c.dpsLocker.Lock()
	defer c.dpsLocker.Unlock()

	_, _ = src.Write(flateTail)
	c.resetFR(src, dict)
	c.lr.R = c.dpsReader
	c.lr.M = c.limit
	c.lr.N = 0
	if _, err := c.dpsBuffer.ReadFrom(&c.lr); err != nil {
		return nil, err
	}
	dst := c.dpsBuffer
	c.dpsBuffer = binaryPool.Get(dst.Len())
	return dst, nil
}

// 剥离压缩输出尾部的同步刷新标记 (00 00 FF FF), RFC7692要求发送前移除
func stripSyncFlushTail(b *bytes.Buffer) {
	if n := b.Len(); n >= flateFlushTailLen {
		if tail := b.Bytes()[n-flateFlushTailLen:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			b.Truncate(n - flateFlushTailLen)
		}
	}
}

// Compress 压缩
func (c *deflater) Compress(src internal.Payload, dst *bytes.Buffer, dict []byte) error {
	c.cpsLocker.Lock()
	defer c.cpsLocker.Unlock()
	if err := compressTo(c.cpsWriter, src, dst, dict); err != nil {
		return err
	}
	stripSyncFlushTail(dst)
	return nil
}

func (c *deflater) CompressBytes(src []byte, dst *bytes.Buffer, dict []byte) error {
	c.cpsLocker.Lock()
	defer c.cpsLocker.Unlock()
	if err := compressToBytes(c.cpsWriter, src, dst, dict); err != nil {
		return err
	}
	stripSyncFlushTail(dst)
	return nil
}

func compressTo(cpsWriter *flate.Writer, r io.WriterTo, w io.Writer, dict []byte) error {
	cpsWriter.ResetDict(w, dict)
	if _, err := r.WriteTo(cpsWriter); err != nil {
		return err
	}
	return cpsWriter.Flush()
}

func compressToBytes(cpsWriter *flate.Writer, src []byte, w io.Writer, dict []byte) error {
	cpsWriter.ResetDict(w, dict)
	if _, err := cpsWriter.Write(src); err != nil {
		return err
	}
	return cpsWriter.Flush()
}

// 滑动窗口
type slideWindow struct {
	enabled bool
	dict    []byte
	size    int
}

// 初始化滑动窗口
func (c *slideWindow) initialize(pool *internal.Pool[[]byte], windowBits int) *slideWindow {
	c.enabled = true
	c.size = internal.BinaryPow(windowBits)
	if pool != nil {
		c.dict = pool.Get()[:0]
	} else {
		c.dict = make([]byte, 0, c.size)
	}
	return c
}

// Write 将数据写入滑动窗口
func (c *slideWindow) Write(p []byte) (int, error) {
	if !c.enabled {
		return 0, nil
	}

	var total = len(p)
	var n = total
	var length = len(c.dict)
	if n+length <= c.size {
		c.dict = append(c.dict, p...)
		return total, nil
	}

	if m := c.size - length; m > 0 {
		c.dict = append(c.dict, p[:m]...)
		p = p[m:]
		n = len(p)
	}

	if n >= c.size {
		copy(c.dict, p[n-c.size:])
		return total, nil
	}

	copy(c.dict, c.dict[n:])
	copy(c.dict[c.size-n:], p)
	return total, nil
}

// 生成请求头
func (c *PermessageDeflate) genRequestHeader() string {
	var options = make([]string, 0, 5)
	options = append(options, internal.PermessageDeflate)
	if !c.ServerContextTakeover {
		options = append(options, internal.ServerNoContextTakeover)
	}
	if !c.ClientContextTakeover {
		options = append(options, internal.ClientNoContextTakeover)
	}
	if c.ServerMaxWindowBits != 15 {
		options = append(options, internal.ServerMaxWindowBits+internal.EQ+strconv.Itoa(c.ServerMaxWindowBits))
	}
	if c.ClientMaxWindowBits != 15 {
		options = append(options, internal.ClientMaxWindowBits+internal.EQ+strconv.Itoa(c.ClientMaxWindowBits))
	} else if c.ClientContextTakeover {
		options = append(options, internal.ClientMaxWindowBits)
	}
	return strings.Join(options, "; ")
}

// 生成响应头
func (c *PermessageDeflate) genResponseHeader() string {
	var options = make([]string, 0, 5)
	options = append(options, internal.PermessageDeflate)
	if !c.ServerContextTakeover {
		options = append(options, internal.ServerNoContextTakeover)
	}
	if !c.ClientContextTakeover {
		options = append(options, internal.ClientNoContextTakeover)
	}
	if c.ServerMaxWindowBits != 15 {
		options = append(options, internal.ServerMaxWindowBits+internal.EQ+strconv.Itoa(c.ServerMaxWindowBits))
	}
	if c.ClientMaxWindowBits != 15 {
		options = append(options, internal.ClientMaxWindowBits+internal.EQ+strconv.Itoa(c.ClientMaxWindowBits))
	}
	return strings.Join(options, "; ")
}

// 压缩拓展协商
// 返回结果中ClientMaxWindowBits为0表示对端未提供client_max_window_bits参数;
// 参数出现但不带值时按RFC7692默认值15处理.
func permessageNegotiation(str string) PermessageDeflate {
	var options = PermessageDeflate{
		ServerContextTakeover: true,
		ClientContextTakeover: true,
		ServerMaxWindowBits:   15,
		ClientMaxWindowBits:   0,
	}

	var ss = internal.Split(str, ";")
	for _, s := range ss {
		var pair = strings.SplitN(s, "=", 2)
		switch pair[0] {
		case internal.PermessageDeflate:
		case internal.ServerNoContextTakeover:
			options.ServerContextTakeover = false
		case internal.ClientNoContextTakeover:
			options.ClientContextTakeover = false
		case internal.ServerMaxWindowBits:
			if len(pair) == 2 {
				x, _ := strconv.Atoi(pair[1])
				x = internal.WithDefault(x, 15)
				options.ServerMaxWindowBits = internal.Min(options.ServerMaxWindowBits, x)
			}
		case internal.ClientMaxWindowBits:
			if len(pair) == 2 {
				x, _ := strconv.Atoi(pair[1])
				x = internal.WithDefault(x, 15)
				options.ClientMaxWindowBits = internal.Min(internal.WithDefault(options.ClientMaxWindowBits, 15), x)
			} else {
				options.ClientMaxWindowBits = internal.WithDefault(options.ClientMaxWindowBits, 15)
			}
		}
	}

	options.ClientMaxWindowBits = internal.SelectValue(options.ClientMaxWindowBits != 0 && options.ClientMaxWindowBits < 8, 8, options.ClientMaxWindowBits)
	options.ServerMaxWindowBits = internal.SelectValue(options.ServerMaxWindowBits < 8, 8, options.ServerMaxWindowBits)
	return options
}

type limitedReader struct {
	R io.Reader
	N int
	M int
}

func (c *limitedReader) Read(p []byte) (n int, err error) {
	n, err = c.R.Read(p)
	c.N += n
	if c.N > c.M {
		return n, internal.CloseMessageTooLarge
	}
	return
}
