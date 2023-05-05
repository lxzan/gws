package gws

import "github.com/lxzan/gws/internal"

var (
	_bpool = internal.NewBufferPool()
	_cps   = new(compressors)
	_dps   = new(decompressors).init()

	JsonCodec = new(jsonCodec)
)
