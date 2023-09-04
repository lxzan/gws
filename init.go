package gws

import "github.com/lxzan/gws/internal"

var (
	myPadding   = frameHeader{}            // 帧头填充物
	staticPool  = internal.NewBufferPool() // 静态缓冲池
	elasticPool = internal.NewBufferPool() // 弹性缓冲池
)
