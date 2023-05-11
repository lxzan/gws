package gws

import (
	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
	"sync"
)

var (
	_bpool           = internal.NewBufferPool()
	_padding         = frameHeader{}
	_compressorPools [12]*sync.Pool
)

func init() {
	var levels = []int{flate.HuffmanOnly, flate.DefaultCompression, flate.NoCompression, flate.BestSpeed, flate.BestCompression}
	for _, level := range levels {
		_compressorPools[level+2] = &sync.Pool{New: func() any {
			return newCompressor(level)
		}}
	}
}
