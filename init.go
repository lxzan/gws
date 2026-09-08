package gws

import "github.com/lxzan/gws/internal"

var (
	framePadding    = frameHeader{}              // 帧头填充物
	defaultLogger   = new(stdLogger)             // 默认日志工具
	bufferThreshold = uint32(256 * 1024)         // buffer 阈值
	binaryPool      = new(internal.BufferPool)   // 内存池
	messagePool     = internal.NewPool(func() *Message { return &Message{} })
)

func init() {
	SetBufferThreshold(bufferThreshold)
}

// SetBufferThreshold 设置 buffer 阈值, x=pow(2,n), 超过 x 字节的 buffer 不会被回收
func SetBufferThreshold(x uint32) {
	bufferThreshold = internal.ToBinaryNumber(x)
	binaryPool = internal.NewBufferPool(128, bufferThreshold)
}
