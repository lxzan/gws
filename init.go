package gws

import "github.com/lxzan/gws/internal"

var (
	framePadding  = frameHeader{}                         // 帧头填充物
	binaryPool    = internal.NewBufferPool(128, 256*1024) // 内存池
	defaultLogger = new(stdLogger)                        // 默认日志工具
)
