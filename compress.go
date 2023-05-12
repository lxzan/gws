package gws

import (
	"encoding/binary"
	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"io"
	"math"
	"sync"
	"sync/atomic"
)

type compressors struct {
	serial      uint64
	size        uint64
	compressors []*compressor
}

func (c *compressors) initialize(num int, level int) *compressors {
	c.size = uint64(internal.ToBinaryNumber(num))
	for i := uint64(0); i < c.size; i++ {
		c.compressors = append(c.compressors, newCompressor(level))
	}
	return c
}

func (c *compressors) Select() *compressor {
	var j = atomic.AddUint64(&c.serial, 1) & (c.size - 1)
	return c.compressors[j]
}

func newCompressor(level int) *compressor {
	fw, _ := flate.NewWriter(nil, level)
	return &compressor{fw: fw, level: level}
}

// 压缩器
type compressor struct {
	sync.Mutex
	level int
	fw    *flate.Writer
}

// Compress 压缩
func (c *compressor) Compress(content []byte, buf *Buffer) error {
	c.Lock()
	defer c.Unlock()

	c.fw.Reset(buf)
	if err := internal.WriteN(c.fw, content, len(content)); err != nil {
		return err
	}
	if err := c.fw.Flush(); err != nil {
		return err
	}
	if n := buf.Len(); n >= 4 {
		compressedContent := buf.Bytes()
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			buf.Truncate(n - 4)
		}
	}
	return nil
}

type decompressors struct {
	serial        uint64
	size          uint64
	decompressors []*decompressor
}

func (c *decompressors) initialize(num int, level int) *decompressors {
	c.size = uint64(internal.ToBinaryNumber(num))
	for i := uint64(0); i < c.size; i++ {
		c.decompressors = append(c.decompressors, newDecompressor())
	}
	return c
}

func (c *decompressors) Select() *decompressor {
	var j = atomic.AddUint64(&c.serial, 1) & (c.size - 1)
	return c.decompressors[j]
}

func newDecompressor() *decompressor {
	return &decompressor{fr: flate.NewReader(nil)}
}

type decompressor struct {
	sync.Mutex
	fr     io.ReadCloser
	buffer [internal.Lv3]byte
}

// Decompress 解压
func (c *decompressor) Decompress(payload *Buffer) (*Buffer, error) {
	c.Lock()
	defer c.Unlock()

	_, _ = payload.Write(internal.FlateTail)
	resetter := c.fr.(flate.Resetter)
	_ = resetter.Reset(payload, nil) // must return a null pointer
	var buf = myBufferPool.Get(payload.Len() * 7 / 2)
	_, err := io.CopyBuffer(buf, c.fr, c.buffer[0:])
	myBufferPool.Put(payload)
	return buf, err
}
