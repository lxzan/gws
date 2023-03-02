package gws

import (
	"sync"
)

type (
	workerQueue struct {
		mu             sync.RWMutex // 锁
		q              []asyncJob   // 任务队列
		maxConcurrency int32        // 最大并发
		curConcurrency int32        // 当前并发
	}

	asyncJob func()
)

// newWorkerQueue 创建一个任务队列
func newWorkerQueue(maxConcurrency int32) *workerQueue {
	c := &workerQueue{
		mu:             sync.RWMutex{},
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
	if n := len(c.q); n == 0 {
		return nil
	}
	var result = c.q[0]
	c.q = c.q[1:]
	c.curConcurrency++
	return result
}

// 递归地执行任务
func (c *workerQueue) do(job asyncJob) {
	job()
	if nextJob := c.getJob(-1); nextJob != nil {
		c.do(nextJob)
	}
}

func (c *workerQueue) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.q)
}

// Push 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) Push(job asyncJob) {
	c.mu.Lock()
	c.q = append(c.q, job)
	c.mu.Unlock()

	if item := c.getJob(0); item != nil {
		go c.do(item)
	}
}
