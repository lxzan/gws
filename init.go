package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
)

var (
	framePadding  = frameHeader{}                // 帧头填充物
	binaryPool    = NewBufferPool(128, 256*1024) // 内存池
	defaultLogger = new(stdLogger)               // 默认日志工具
)

type BufferPool interface {
	Get(n int) *bytes.Buffer
	Put(b *bytes.Buffer)
}

func NewBufferPool(minSize, maxSize uint32) BufferPool {
	return internal.NewBufferPool(minSize, maxSize)
}

func SetBufferPool(p BufferPool) { binaryPool = p }
