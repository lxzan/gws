package gws

import "github.com/lxzan/gws/internal"

var (
	framePadding    = frameHeader{}            // 帧头填充物
	defaultLogger   = new(stdLogger)           // 默认日志工具
	bufferThreshold = uint32(256 * 1024)       // buffer阈值
	binaryPool      = new(internal.BufferPool) // 内存池
)

func init() {
	SetBufferThreshold(bufferThreshold)
}

// SetBufferThreshold 设置buffer阈值, x=pow(2,n), 超过x个字节的buffer不会被回收
// Set the buffer threshold, x=pow(2,n), that buffers larger than x bytes are not reclaimed.
func SetBufferThreshold(x uint32) {
	bufferThreshold = internal.ToBinaryNumber(x)
	binaryPool = internal.NewBufferPool(128, bufferThreshold)
}
