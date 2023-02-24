package gws

import (
	"github.com/lxzan/gws/internal"
	"sync"
	"sync/atomic"
)

type (
	workerQueue struct {
		mu             *sync.Mutex // 锁
		q              []asyncJob  // 任务队列
		maxConcurrency int64       // 最大并发
		curConcurrency int64       // 当前并发
	}

	asyncJob struct {
		Args *Conn
		Do   func(args *Conn) error
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
		q:              make([]asyncJob, 0),
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
		_writeQueue.AddJob(asyncJob{Args: conn, Do: doWriteAsync})
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

// WriteAsync
// 异步写入消息, 适合广播等需要非阻塞的场景
// asynchronous write messages, suitable for non-blocking scenarios such as broadcasting
func (c *Conn) WriteAsync(opcode Opcode, payload []byte) {
	c.wmq.Push(c, messageWrapper{
		opcode:  opcode,
		payload: payload,
	})
}

// 写入并清空消息
func doWriteAsync(conn *Conn) error {
	if conn.wmq.Len() == 0 {
		return nil
	}

	conn.wmu.Lock()
	err := conn.wmq.Range(func(msg messageWrapper) error {
		if atomic.LoadUint32(&conn.closed) == 1 {
			return internal.ErrConnClosed
		}
		return conn.writePublic(msg.opcode, msg.payload)
	})
	conn.wmu.Unlock()

	if err != nil {
		conn.emitError(err)
	}
	return err
}
