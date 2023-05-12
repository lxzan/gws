package gws

import "github.com/lxzan/gws/internal"

var (
	myBufferPool = internal.NewBufferPool()
	myPadding    = frameHeader{}
)
