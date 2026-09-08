package gws

import (
	"sync"

	"github.com/lxzan/gws/internal"
)

type (
	// workerQueue 任务队列
	workerQueue struct {
		mu             sync.Mutex               // 互斥锁
		q              internal.Deque[asyncJob] // 双端队列, 存储异步任务
		maxConcurrency int32                    // 最大并发数
		curConcurrency int32                    // 当前并发数
	}

	// asyncJob 异步任务
	asyncJob func()
)

// newWorkerQueue 创建任务队列
func newWorkerQueue(maxConcurrency int32) *workerQueue {
	c := &workerQueue{
		mu:             sync.Mutex{},
		maxConcurrency: maxConcurrency,
		curConcurrency: 0,
	}
	return c
}

// getJob 获取一个任务
func (c *workerQueue) getJob(newJob asyncJob, delta int32) asyncJob {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newJob != nil {
		c.q.PushBack(newJob)
	}
	c.curConcurrency += delta
	if c.curConcurrency >= c.maxConcurrency {
		return nil
	}
	var job = c.q.PopFront()
	if job == nil {
		return nil
	}
	c.curConcurrency++
	return job
}

// execute 执行单个任务, 捕获 panic 防止 worker 协程退出导致队列停摆
func (c *workerQueue) execute(job asyncJob) {
	defer func() {
		_ = recover()
	}()
	job()
}

// do 循环执行任务
func (c *workerQueue) do(job asyncJob) {
	for job != nil {
		c.execute(job)
		job = c.getJob(nil, -1)
	}
}

// Push 追加任务, 有资源空闲时立即执行
func (c *workerQueue) Push(job asyncJob) {
	if nextJob := c.getJob(job, 0); nextJob != nil {
		go c.do(nextJob)
	}
}

type channel chan struct{}

func (c channel) add() { c <- struct{}{} }

func (c channel) done() { <-c }

func (c channel) Go(m *Message, f func(*Message) error) error {
	c.add()
	go func() {
		_ = f(m)
		c.done()
	}()
	return nil
}
