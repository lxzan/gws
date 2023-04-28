package gws

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"math"
)

func newCompressor(level int) *compressor {
	fw, _ := flate.NewWriter(nil, level)
	return &compressor{fw: fw}
}

// 压缩器
type compressor struct {
	fw *flate.Writer
}

// Compress 压缩
func (c *compressor) Compress(content []byte, buf *bytes.Buffer) error {
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

func newDecompressor() *decompressor { return &decompressor{fr: flate.NewReader(nil)} }

type decompressor struct {
	fr io.ReadCloser
}

// Decompress 解压
func (c *decompressor) Decompress(payload *bytes.Buffer) (*bytes.Buffer, error) {
	_, _ = payload.Write(internal.FlateTail)
	resetter := c.fr.(flate.Resetter)
	_ = resetter.Reset(payload, nil) // must return a null pointer

	var buf = _bpool.Get(3 * payload.Len())
	_, err := io.Copy(buf, c.fr)
	_bpool.Put(payload)
	return buf, err
}
