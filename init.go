package gws

import "github.com/lxzan/gws/internal"

var (
	// framePadding 用于填充帧头
	// framePadding is used to pad the frame header
	framePadding = frameHeader{}

	// binaryPool 是一个缓冲池，用于管理二进制数据缓冲区
	// binaryPool is a buffer pool used to manage binary data buffers
	// 参数 128 表示缓冲区的初始大小，256*1024 表示缓冲区的最大大小
	// The parameter 128 represents the initial size of the buffer, and 256*1024 represents the maximum size of the buffer
	binaryPool = internal.NewBufferPool(128, 256*1024)

	// defaultLogger 是默认的日志工具
	// defaultLogger is the default logging tool
	defaultLogger = new(stdLogger)
)
