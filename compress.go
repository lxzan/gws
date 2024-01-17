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

// FlateTail Add four bytes as specified in RFC
// Add final block to squelch unexpected EOF error from flate reader.
var flateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

type deflater interface {
	Compress(src []byte, dst *bytes.Buffer, dict []byte) error
	Decompress(src *bytes.Buffer, dict []byte) (*bytes.Buffer, error)
}

type deflaterPool struct {
	serial uint64
	num    uint64
	pool   []deflater
}

func (c *deflaterPool) initialize(options PermessageDeflate) *deflaterPool {
	c.num = uint64(options.PoolSize)
	for i := uint64(0); i < c.num; i++ {
		var d = &deflaterS{
			dpsReader: flate.NewReader(nil),
			dpsBuffer: bytes.NewBuffer(nil),
		}
		if options.ClientContextTakeover {
			d.cpsWriter, _ = flate.NewWriterWindow(nil, internal.BinaryPow(options.ClientMaxWindowBits))
		} else {
			if options.ClientMaxWindowBits == 15 {
				d.cpsWriter, _ = flate.NewWriter(nil, options.Level)
			} else {
				d.cpsWriter, _ = flate.NewWriterWindow(nil, internal.BinaryPow(options.ClientMaxWindowBits))
			}
		}
		c.pool = append(c.pool, d)
	}
	return c
}

func (c *deflaterPool) Select() deflater {
	var j = atomic.AddUint64(&c.serial, 1) & (c.num - 1)
	return c.pool[j]
}

// 服务端的压缩编码器
type deflaterS struct {
	dpsLocker sync.Mutex
	dpsBuffer *bytes.Buffer
	dpsReader io.ReadCloser
	cpsLocker sync.Mutex
	cpsWriter *flate.Writer
}

func (c *deflaterS) resetFR(r io.Reader, dict []byte) {
	resetter := c.dpsReader.(flate.Resetter)
	_ = resetter.Reset(r, dict) // must return a null pointer
	if c.dpsBuffer.Cap() > 256*1024 {
		c.dpsBuffer = bytes.NewBuffer(nil)
	}
	c.dpsBuffer.Reset()
}

// Decompress 解压
func (c *deflaterS) Decompress(src *bytes.Buffer, dict []byte) (*bytes.Buffer, error) {
	c.dpsLocker.Lock()
	defer c.dpsLocker.Unlock()

	_, _ = src.Write(flateTail)
	c.resetFR(src, dict)
	if _, err := c.dpsReader.(io.WriterTo).WriteTo(c.dpsBuffer); err != nil {
		return nil, err
	}
	var dst = binaryPool.Get(c.dpsBuffer.Len())
	_, _ = c.dpsBuffer.WriteTo(dst)
	return dst, nil
}

// Compress 压缩
func (c *deflaterS) Compress(src []byte, dst *bytes.Buffer, dict []byte) error {
	c.cpsLocker.Lock()
	defer c.cpsLocker.Unlock()

	c.cpsWriter.ResetDict(dst, dict)
	if err := internal.WriteN(c.cpsWriter, src); err != nil {
		return err
	}
	if err := c.cpsWriter.Flush(); err != nil {
		return err
	}
	if n := dst.Len(); n >= 4 {
		compressedContent := dst.Bytes()
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			dst.Truncate(n - 4)
		}
	}
	return nil
}

// 客户端的压缩编码器
type deflaterC struct {
	mu                    sync.Mutex    // 写锁
	serverContextTakeover bool          // 解压器是否开启上下文接管
	clientContextTakeover bool          // 压缩器是否开启上下文接管
	serverMaxWindowBits   int           // 解压器的滑动窗口大小(8~15)
	clientMaxWindowBits   int           // 压缩器的滑动窗口大小(8~15)
	dpsBuffer             *bytes.Buffer // 解压器缓冲
	dpsReader             io.ReadCloser // 解压器
	cpsWriter             *flate.Writer // 压缩器
}

func (c *deflaterC) initialize(level int, str string) error {
	c.dpsBuffer = bytes.NewBuffer(nil)
	c.serverContextTakeover = true
	c.clientContextTakeover = true
	c.serverMaxWindowBits = 15
	c.clientMaxWindowBits = 15
	var ss = internal.Split(str, ";")
	for _, s := range ss {
		var pair = strings.SplitN(s, "=", 2)
		switch pair[0] {
		case internal.PermessageDeflate:
		case internal.ServerNoContextTakeover:
			c.serverContextTakeover = false
		case internal.ClientNoContextTakeover:
			c.clientContextTakeover = false
		case internal.ServerMaxWindowBits:
			if len(pair) == 2 {
				x, _ := strconv.Atoi(pair[1])
				x = internal.WithDefault(x, 15)
				c.serverMaxWindowBits = internal.Min(c.serverMaxWindowBits, x)
			}
		case internal.ClientMaxWindowBits:
			if len(pair) == 2 {
				x, _ := strconv.Atoi(pair[1])
				x = internal.WithDefault(x, 15)
				c.clientMaxWindowBits = internal.Min(c.clientMaxWindowBits, x)
			}
		}
	}

	if c.clientContextTakeover {
		c.cpsWriter, _ = flate.NewWriterWindow(nil, internal.BinaryPow(c.clientMaxWindowBits))
	} else {
		if c.clientMaxWindowBits == 15 {
			c.cpsWriter, _ = flate.NewWriter(nil, level)
		} else {
			c.cpsWriter, _ = flate.NewWriterWindow(nil, internal.BinaryPow(c.clientMaxWindowBits))
		}
	}

	c.dpsReader = flate.NewReader(nil)

	if c.clientMaxWindowBits < 8 || c.serverMaxWindowBits < 8 {
		return ErrCompressionNegotiation
	}

	return nil
}

func (c *deflaterC) Compress(src []byte, dst *bytes.Buffer, dict []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cpsWriter.ResetDict(dst, dict)
	if err := internal.WriteN(c.cpsWriter, src); err != nil {
		return err
	}
	if err := c.cpsWriter.Flush(); err != nil {
		return err
	}
	if n := dst.Len(); n >= 4 {
		compressedContent := dst.Bytes()
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			dst.Truncate(n - 4)
		}
	}
	return nil
}

func (c *deflaterC) resetFR(r io.Reader, dict []byte) {
	resetter := c.dpsReader.(flate.Resetter)
	_ = resetter.Reset(r, dict) // must return a null pointer
	if c.dpsBuffer.Cap() > 256*1024 {
		c.dpsBuffer = bytes.NewBuffer(nil)
	}
	c.dpsBuffer.Reset()
}

func (c *deflaterC) Decompress(src *bytes.Buffer, dict []byte) (*bytes.Buffer, error) {
	_, _ = src.Write(flateTail)
	c.resetFR(src, dict)
	if _, err := c.dpsReader.(io.WriterTo).WriteTo(c.dpsBuffer); err != nil {
		return nil, err
	}
	var dst = binaryPool.Get(c.dpsBuffer.Len())
	_, _ = c.dpsBuffer.WriteTo(dst)
	return dst, nil
}

type slideWindow struct {
	enabled bool
	dict    []byte
	size    int
}

func (c *slideWindow) initialize(windowBits int) *slideWindow {
	c.enabled = true
	c.size = internal.BinaryPow(windowBits)
	c.dict = make([]byte, 0, c.size)
	return c
}

func (c *slideWindow) Write(p []byte) {
	if !c.enabled {
		return
	}

	var n = len(p)
	var length = len(c.dict)
	if n+length <= c.size {
		c.dict = append(c.dict, p...)
		return
	}

	var m = c.size - length
	c.dict = append(c.dict, p[:m]...)
	p = p[m:]
	n = len(p)

	if n >= c.size {
		copy(c.dict, p[n-c.size:])
		return
	}

	copy(c.dict, c.dict[n:])
	copy(c.dict[c.size-n:], p)
}

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
		options = append(options, "server_max_window_bits="+strconv.Itoa(c.ServerMaxWindowBits))
	}
	if c.ClientMaxWindowBits != 15 {
		options = append(options, "client_max_window_bits="+strconv.Itoa(c.ClientMaxWindowBits))
	} else if c.ServerContextTakeover && c.ClientContextTakeover {
		options = append(options, internal.ClientMaxWindowBits)
	}
	return strings.Join(options, "; ")
}

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
		options = append(options, "server_max_window_bits="+strconv.Itoa(c.ServerMaxWindowBits))
	}
	if c.ClientMaxWindowBits != 15 {
		options = append(options, "client_max_window_bits="+strconv.Itoa(c.ClientMaxWindowBits))
	}
	return strings.Join(options, "; ")
}
