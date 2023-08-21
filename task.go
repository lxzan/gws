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

func (c *workerQueue) pop() asyncJob {
	if len(c.q) == 0 {
		return nil
	}
	var job = c.q[0]
	c.q = c.q[1:]
	return job
}

// 获取一个任务
func (c *workerQueue) getJob(newJob asyncJob, delta int32) asyncJob {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newJob != nil {
		c.q = append(c.q, newJob)
	}
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
		job = c.getJob(nil, -1)
	}
}

// Push 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) Push(job asyncJob) {
	if nextJob := c.getJob(job, 0); nextJob != nil {
		go c.do(nextJob)
	}
}

type channel chan struct{}

func (c channel) add() { c <- struct{}{} }

func (c channel) done() { <-c }

func (c channel) Go(f func()) {
	c.add()
	go func() {
		f()
		c.done()
	}()
}
