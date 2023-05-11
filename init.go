package gws

import (
	"github.com/lxzan/gws/internal"
)

var (
	myBufferPool = internal.NewBufferPool()
	myPadding    = frameHeader{}
	myCompressor = new(compressors)
)

func init() {
	SetFlateCompressor(8, defaultCompressLevel)
}
