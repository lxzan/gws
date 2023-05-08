package gws

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"sync"
	"sync/atomic"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

const numCompressor = 32

type compressors struct {
	sync.RWMutex
	serial      uint64
	compressors [12][numCompressor]*compressor
}

func (c *compressors) Select(level int) *compressor {
	var i = level + 2
	var j = atomic.AddUint64(&c.serial, 1) & (numCompressor - 1)
	c.RLock()
	var cps = c.compressors[i][j]
	c.RUnlock()

	if cps == nil {
		c.Lock()
		cps = newCompressor(level)
		c.compressors[i][j] = cps
		c.Unlock()
	}
	return cps
}

func newCompressor(level int) *compressor {
	fw, _ := flate.NewWriter(nil, level)
	return &compressor{fw: fw, mu: &sync.Mutex{}}
}

// 压缩器
type compressor struct {
	mu *sync.Mutex
	fw *flate.Writer
}

// Compress 压缩
func (c *compressor) Compress(content []byte, buf *bytes.Buffer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *compressor) CompressAny(codec Codec, v interface{}, buf *bytes.Buffer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fw.Reset(buf)
	if err := codec.NewEncoder(c.fw).Encode(v); err != nil {
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
	decompressors [numCompressor]*decompressor
}

func (c *decompressors) init() *decompressors {
	for i, _ := range c.decompressors {
		c.decompressors[i] = newDecompressor()
	}
	return c
}

func (c *decompressors) Select() *decompressor {
	var index = atomic.AddUint64(&c.serial, 1) & (numCompressor - 1)
	return c.decompressors[index]
}

func newDecompressor() *decompressor {
	return &decompressor{fr: flate.NewReader(nil), mu: &sync.Mutex{}}
}

type decompressor struct {
	mu     *sync.Mutex
	fr     io.ReadCloser
	buffer [internal.Lv3]byte
}

// Decompress 解压
func (c *decompressor) Decompress(payload *bytes.Buffer) (*bytes.Buffer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, _ = payload.Write(internal.FlateTail)
	resetter := c.fr.(flate.Resetter)
	_ = resetter.Reset(payload, nil) // must return a null pointer

	var buf = _bpool.Get(3 * payload.Len())
	_, err := io.CopyBuffer(buf, c.fr, c.buffer[0:])
	_bpool.Put(payload)
	return buf, err
}
