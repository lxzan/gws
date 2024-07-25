package gws

import "github.com/lxzan/gws/internal"

var (
	framePadding  = frameHeader{}            // 帧头填充物
	binaryPool    = new(internal.BufferPool) // 内存池
	defaultLogger = new(stdLogger)           // 默认日志工具
)

func init() {
	SetBufferPool(256 * 1024)
}

// SetBufferPool set up the memory pool, any memory that exceeds maxSize will not be reclaimed.
// 设置内存池, 超过maxSize将不会被回收.
func SetBufferPool(maxSize uint32) {
	binaryPool = internal.NewBufferPool(128, maxSize)
}
