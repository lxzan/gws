package gws

import "github.com/lxzan/gws/internal"

var _pool *internal.BufferPool

func init() {
	_pool = internal.NewBufferPool()
}
