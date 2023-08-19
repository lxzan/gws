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
		offset         int        // 偏移量
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

func (c *workerQueue) pop() asyncJob {
	var n = len(c.q) - c.offset
	if n == 0 {
		return nil
	}
	job := c.q[c.offset]
	c.q[c.offset] = nil
	c.offset++
	if n == 1 {
		c.offset = 0
		c.q = c.q[:0]
		if cap(c.q) > 256 {
			c.q = nil
		}
	}
	return job
}

// 获取一个任务
func (c *workerQueue) getJob(delta int32) asyncJob {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.curConcurrency += delta
	if c.curConcurrency >= c.maxConcurrency {
		return nil
	}
	var job = c.pop()
	if job == nil {
		return nil
	}
	c.curConcurrency++
	return job
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
