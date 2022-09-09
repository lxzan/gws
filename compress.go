package websocket

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"math"
	"sync"
	"sync/atomic"
)

type compressors struct {
	serial uint64
	n      uint64
	cps    []*compressor
}

func newCompressors(n int, wSize int) *compressors {
	var c = make([]*compressor, 0, n)
	for i := 0; i < n; i++ {
		cps := newCompressor(wSize)
		c = append(c, cps)
	}
	return &compressors{cps: c, n: uint64(n)}
}

func (c *compressors) Select() *compressor {
	next := atomic.AddUint64(&c.serial, 1)
	idx := next & (c.n - 1)
	return c.cps[idx]
}

func newCompressor(writeBufferSize int) *compressor {
	var c compressor
	fw, _ := flate.NewWriter(nil, flate.BestSpeed)
	c.fw = fw
	c.writeBuffer = bytes.NewBuffer(nil)
	c.wSize = writeBufferSize
	return &c
}

// 压缩器
type compressor struct {
	sync.Mutex
	wSize       int
	writeBuffer *bytes.Buffer
	fw          *flate.Writer
}

// 压缩并构建WriteFrame
func (c *compressor) Compress(content []byte) ([]byte, error) {
	c.Lock()
	if c.writeBuffer.Cap() > c.wSize {
		c.writeBuffer = bytes.NewBuffer(nil)
	}

	c.writeBuffer.Reset()
	c.fw.Reset(c.writeBuffer)
	if err := writeN(c.fw, content, len(content)); err != nil {
		return nil, err
	}
	if err := c.fw.Flush(); err != nil {
		return nil, err
	}

	contentLength := c.writeBuffer.Len()
	compressedContent := c.writeBuffer.Bytes()
	if n := c.writeBuffer.Len(); n >= 4 {
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			contentLength -= 4
			compressedContent = compressedContent[:n-4]
		}
	}

	return compressedContent, nil
}

type decompressors struct {
	serial uint64
	n      uint64
	dps    []*decompressor
}

func newDecompressors(n int, rSize int) *decompressors {
	var c = make([]*decompressor, 0, n)
	for i := 0; i < n; i++ {
		dps := newDecompressor(rSize)
		c = append(c, dps)
	}
	return &decompressors{dps: c, n: uint64(n)}
}

func (c *decompressors) Select() *decompressor {
	next := atomic.AddUint64(&c.serial, 1)
	idx := next & (c.n - 1)
	return c.dps[idx]
}

func newDecompressor(readBufferSize int) *decompressor {
	var c decompressor
	c.fr = flate.NewReader(nil)
	c.readBuffer = bytes.NewBuffer(nil)
	c.rSize = readBufferSize
	return &c
}

type decompressor struct {
	sync.Mutex
	rSize      int
	readBuffer *bytes.Buffer
	fr         io.ReadCloser
}

// 解压
func (c *decompressor) Decompress(content *bytes.Buffer) (*bytes.Buffer, error) {
	c.Lock()
	defer c.Unlock()

	if c.readBuffer.Cap() > c.rSize {
		c.readBuffer = bytes.NewBuffer(nil)
	}

	_, _ = content.Write(internal.FlateTail)
	resetter := c.fr.(flate.Resetter)
	if err := resetter.Reset(content, nil); err != nil {
		return nil, err
	}

	var dst = _pool.Get(content.Len())
	_, err := io.Copy(dst, c.fr)
	_pool.Put(content)
	return dst, err
}
