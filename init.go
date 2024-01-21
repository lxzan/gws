package gws

import "github.com/lxzan/gws/internal"

var (
	framePadding  = frameHeader{}            // 帧头填充物
	binaryPool    = internal.NewBufferPool() // 缓冲池
	defaultLogger = new(stdLogger)           // 默认日志工具
)
