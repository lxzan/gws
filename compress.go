package gws

import (
	"compress/flate"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"math"
)

func newCompressor(level int) *compressor {
	fw, _ := flate.NewWriter(nil, level)
	return &compressor{
		writeBuffer: internal.NewBuffer(nil),
		fw:          fw,
	}
}

// 压缩器
type compressor struct {
	writeBuffer *internal.Buffer
	fw          *flate.Writer
}

// Compress 压缩
func (c *compressor) Compress(content []byte) ([]byte, error) {
	if c.writeBuffer.Cap() > internal.Lv3 {
		c.writeBuffer = internal.NewBuffer(nil)
	}

	c.writeBuffer.Reset()
	c.fw.Reset(c.writeBuffer)
	if err := writeN(c.fw, content, len(content)); err != nil {
		return nil, err
	}
	if err := c.fw.Flush(); err != nil {
		return nil, err
	}

	compressedContent := c.writeBuffer.Bytes()
	if n := c.writeBuffer.Len(); n >= 4 {
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			compressedContent = compressedContent[:n-4]
		}
	}

	return compressedContent, nil
}

func newDecompressor() *decompressor {
	return &decompressor{
		fr: flate.NewReader(nil),
	}
}

type decompressor struct {
	fr io.ReadCloser
}

// Decompress 解压
func (c *decompressor) Decompress(payload *internal.Buffer) (*internal.Buffer, error) {
	_, _ = payload.Write(internal.FlateTail)
	resetter := c.fr.(flate.Resetter)
	if err := resetter.Reset(payload, nil); err != nil {
		return nil, err
	}

	var buf = _pool.Get(payload.Len())
	_, err := io.Copy(buf, c.fr)
	_pool.Put(payload)
	return buf, err
}
