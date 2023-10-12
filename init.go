package gws

import "github.com/lxzan/gws/internal"

var (
	myPadding  = frameHeader{}            // 帧头填充物
	binaryPool = internal.NewBufferPool() // 缓冲池

	StdLogger = new(stdLogger) // 标准日志输出
)
