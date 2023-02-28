package gws

import (
	"github.com/lxzan/gws/internal"
	"sync"
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
func (c *workerQueue) getJob(delta int64) interface{} {
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
	_ = job.Do(job.Args)
	if nextJob := c.getJob(-1); nextJob != nil {
		c.do(nextJob.(asyncJob))
	}
}

// AddJob 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) AddJob(job asyncJob) {
	c.mu.Lock()
	c.q = append(c.q, job)
	c.mu.Unlock()
	if item := c.getJob(0); item != nil {
		go c.do(item.(asyncJob))
	}
}

func newMessageQueue(capacity int) *messageQueue {
	var mq = new(messageQueue)
	mq.cap = capacity
	return mq
}

type messageQueue struct {
	sync.Mutex
	cap  int
	data []messageWrapper
}

// 追加一条消息
// 如果容量已满消息会被抛弃并返回错误
func (c *messageQueue) Push(opcode Opcode, payload []byte) error {
	c.Lock()
	defer c.Unlock()
	if len(c.data) >= c.cap {
		return internal.ErrWriteMessageQueueCapFull
	}
	c.data = append(c.data, messageWrapper{
		opcode:  opcode,
		payload: payload,
	})
	return nil
}

// 取出所有消息
func (c *messageQueue) PopAll() []messageWrapper {
	c.Lock()
	defer c.Unlock()
	if len(c.data) == 0 {
		return nil
	}
	msgs := c.data
	c.data = []messageWrapper{}
	return msgs
}
