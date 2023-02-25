package gws

import (
	"context"
	"sync"
	"time"
)

type (
	workerQueue struct {
		mu             *sync.Mutex // 锁
		q              []asyncJob  // 任务队列
		maxConcurrency int64       // 最大并发
		curConcurrency int64       // 当前并发
	}

	asyncJob struct {
		Args interface{}
		Do   func(args interface{}) error
	}

	messageWrapper struct {
		opcode  Opcode
		payload []byte
	}
)

// newWorkerQueue 创建一个任务队列
func newWorkerQueue(maxConcurrency int64) *workerQueue {
	c := &workerQueue{
		mu:             &sync.Mutex{},
		maxConcurrency: maxConcurrency,
		curConcurrency: 0,
	}
	return c
}

// 获取一个任务
func (c *workerQueue) getJob() interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.curConcurrency >= c.maxConcurrency {
		return nil
	}
	if n := len(c.q); n == 0 {
		return nil
	}
	var result = c.q[0]
	c.q = c.q[1:]
	c.curConcurrency++
	return result
}

// 并发减一
func (c *workerQueue) decrease() {
	c.mu.Lock()
	c.curConcurrency--
	c.mu.Unlock()
}

// 递归地执行任务
func (c *workerQueue) do(job asyncJob) {
	_ = job.Do(job.Args)
	c.decrease()
	if nextJob := c.getJob(); nextJob != nil {
		c.do(nextJob.(asyncJob))
	}
}

// AddJob 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) AddJob(job asyncJob) {
	c.mu.Lock()
	c.q = append(c.q, job)
	c.mu.Unlock()
	if item := c.getJob(); item != nil {
		go c.do(item.(asyncJob))
	}
}

// 获取当前并发
func (c *workerQueue) getCurConcurrency() int64 {
	c.mu.Lock()
	num := c.curConcurrency
	c.mu.Unlock()
	return num
}

func (c *workerQueue) Wait(timeout time.Duration) {
	if c.getCurConcurrency() == 0 {
		return
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer func() {
		ticker.Stop()
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.getCurConcurrency() == 0 {
				return
			}
		}
	}
}

type writeQueue struct {
	sync.RWMutex
	data []messageWrapper
}

func (c *writeQueue) Len() int {
	c.RLock()
	n := len(c.data)
	c.RUnlock()
	return n
}

func (c *writeQueue) Push(v messageWrapper) {
	c.Lock()
	c.data = append(c.data, v)
	c.Unlock()
}

func (c *writeQueue) Pop() messageWrapper {
	msg := c.data[0]
	c.data = c.data[1:]
	return msg
}
