package gws

import "github.com/lxzan/gws/internal"

const defaultAsyncWriteConcurrency = 128

var (
	// task queue for async write
	_writeQueue = newWorkerQueue(defaultAsyncWriteConcurrency)

	// buffer pool
	_bpool = internal.NewBufferPool()
)

func SetMaxConcurrencyForWriteQueue(num int64) {
	if num > 0 {
		_writeQueue.maxConcurrency = num
	}
}
