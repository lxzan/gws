package gws

import (
	"bytes"
	"encoding/binary"
	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"io"
	"math"
	"sync"
	"sync/atomic"
)

var numCompressor = uint64(8)

// SetFlateCompressor 设置压缩器数量和压缩级别
// num越大锁竞争概率越小, 但是会耗费大量内存, 取值需要权衡; 对于websocket server, 推荐num=128; 对于client, 推荐不修改使用默认值;
// 推荐level=flate.BestSpeed
func SetFlateCompressor(num int, level int) {
	numCompressor = uint64(internal.ToBinaryNumber(num))
	myCompressor = new(compressors)
	for i := uint64(0); i < numCompressor; i++ {
		myCompressor.compressors = append(myCompressor.compressors, newCompressor(level))
	}
}

type compressors struct {
	serial      uint64
	compressors []*compressor
}

func (c *compressors) Select() *compressor {
	var j = atomic.AddUint64(&c.serial, 1) & (numCompressor - 1)
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
func (c *compressor) Compress(content []byte, buf *bytes.Buffer) error {
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

func newDecompressor() *decompressor {
	return &decompressor{fr: flate.NewReader(nil)}
}

type decompressor struct {
	fr     io.ReadCloser
	buffer [internal.Lv2]byte
}

// Decompress 解压
func (c *decompressor) Decompress(payload *bytes.Buffer) (*bytes.Buffer, error) {
	_, _ = payload.Write(internal.FlateTail)
	resetter := c.fr.(flate.Resetter)
	_ = resetter.Reset(payload, nil) // must return a null pointer
	var buf = myBufferPool.Get(3 * payload.Len())
	_, err := io.CopyBuffer(buf, c.fr, c.buffer[0:])
	myBufferPool.Put(payload)
	return buf, err
}
