package gws

import (
	"sync"
)

type (
	workerQueue struct {
		mu             *sync.Mutex // 锁
		q              []writeJob  // 任务队列
		maxConcurrency int64       // 最大并发
		curConcurrency int64       // 当前并发
	}

	writeJob struct {
		Args *Conn
		Do   func(args *Conn) error
	}

	messageWrapper struct {
		opcode  Opcode
		payload []byte
	}
)

// newWorkerQueue 创建一个工作队列
func newWorkerQueue(maxConcurrency int64) *workerQueue {
	c := &workerQueue{
		mu:             &sync.Mutex{},
		q:              make([]writeJob, 0),
		maxConcurrency: maxConcurrency,
		curConcurrency: 0,
	}
	return c
}

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

func (c *workerQueue) decrease() {
	c.mu.Lock()
	c.curConcurrency--
	c.mu.Unlock()
}

func (c *workerQueue) do(job writeJob) {
	job.Args.emitError(job.Do(job.Args))
	c.decrease()
	if nextJob := c.getJob(); nextJob != nil {
		c.do(nextJob.(writeJob))
	}
}

// AddJob 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) AddJob(job writeJob) {
	c.mu.Lock()
	c.q = append(c.q, job)
	c.mu.Unlock()
	if item := c.getJob(); item != nil {
		go c.do(item.(writeJob))
	}
}

func newMessageQueue() messageQueue {
	return messageQueue{
		mu:   &sync.RWMutex{},
		data: []messageWrapper{},
	}
}

type messageQueue struct {
	mu   *sync.RWMutex
	data []messageWrapper
}

func (c *messageQueue) Len() int {
	c.mu.RLock()
	n := len(c.data)
	c.mu.RUnlock()
	return n
}

func (c *messageQueue) Push(conn *Conn, m messageWrapper) {
	c.mu.Lock()
	c.data = append(c.data, m)
	if n := len(c.data); n == 1 {
		_writeQueue.AddJob(writeJob{Args: conn, Do: doWriteAsync})
	}
	c.mu.Unlock()
}

func (c *messageQueue) Range(f func(msg messageWrapper) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, _ := range c.data {
		if err := f(c.data[i]); err != nil {
			return err
		}
	}
	c.data = c.data[:0]
	return nil
}
