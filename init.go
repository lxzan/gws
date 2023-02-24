package gws

import "github.com/lxzan/gws/internal"

const defaultAsyncWriteConcurrency = 128

var (
	// task queue for async write
	_writeQueue = newWorkerQueue(defaultAsyncWriteConcurrency)

	// buffer pool
	_bpool = internal.NewBufferPool()
)

// set max concurrent goroutines for write queue
func SetGoLimit(num int64) {
	if num > 0 {
		_writeQueue.maxConcurrency = num
	}
}
