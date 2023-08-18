package gws

import (
	"sync"
)

type (
	workerQueue struct {
		mu             sync.Mutex // 锁
		q              []asyncJob // 任务队列
		maxConcurrency int32      // 最大并发
		curConcurrency int32      // 当前并发
	}

	asyncJob func()
)

// newWorkerQueue 创建一个任务队列
func newWorkerQueue(maxConcurrency int32) *workerQueue {
	c := &workerQueue{
		mu:             sync.Mutex{},
		maxConcurrency: maxConcurrency,
		curConcurrency: 0,
	}
	return c
}

// 获取一个任务
func (c *workerQueue) getJob(delta int32) asyncJob {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.curConcurrency += delta
	if c.curConcurrency >= c.maxConcurrency {
		return nil
	}
	if len(c.q) == 0 {
		return nil
	}
	var result = c.q[0]
	c.q = c.q[1:]
	if len(c.q) == 0 && cap(c.q) >= 128 {
		c.q = nil
	}
	c.curConcurrency++
	return result
}

// 循环执行任务
func (c *workerQueue) do(job asyncJob) {
	for job != nil {
		job()
		job = c.getJob(-1)
	}
}

// Push 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) Push(job asyncJob) {
	c.mu.Lock()
	c.q = append(c.q, job)
	c.mu.Unlock()
	if job := c.getJob(0); job != nil {
		go c.do(job)
	}
}
