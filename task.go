package gws

import (
	"sync"

	"github.com/lxzan/gws/internal"
)

type (
	// 任务队列
	// Task queue
	workerQueue struct {
		// mu 互斥锁
		// mutex
		mu sync.Mutex

		// q 双端队列，用于存储异步任务
		// double-ended queue to store asynchronous jobs
		q internal.Deque[asyncJob]

		// maxConcurrency 最大并发数
		// maximum concurrency
		maxConcurrency int32

		// curConcurrency 当前并发数
		// current concurrency
		curConcurrency int32
	}

	// 异步任务
	// Asynchronous job
	asyncJob func()
)

// 创建一个任务队列
// Creates a task queue
func newWorkerQueue(maxConcurrency int32) *workerQueue {
	c := &workerQueue{
		mu:             sync.Mutex{},
		maxConcurrency: maxConcurrency,
		curConcurrency: 0,
	}
	return c
}

// 获取一个任务
// Retrieves a job from the worker queue
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

// 循环执行任务
// Do continuously executes jobs in the worker queue
func (c *workerQueue) do(job asyncJob) {
	for job != nil {
		job()
		job = c.getJob(nil, -1)
	}
}

// Push 追加任务, 有资源空闲的话会立即执行
// Adds a job to the queue and executes it immediately if resources are available
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
