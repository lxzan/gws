package gws

import (
	"bytes"
	"sync"
)

type (
	workerQueue struct {
		mu             sync.Mutex // 锁
		q              heap       // 任务队列
		maxConcurrency int32      // 最大并发
		curConcurrency int32      // 当前并发
	}

	asyncJob struct {
		serial  int
		socket  *Conn
		frame   *bytes.Buffer
		execute func(conn *Conn, buffer *bytes.Buffer) error
	}
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
func (c *workerQueue) getJob(newJob *asyncJob, delta int32) *asyncJob {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newJob != nil {
		c.q.Push(newJob)
	}
	c.curConcurrency += delta
	if c.curConcurrency >= c.maxConcurrency {
		return nil
	}
	var job = c.q.Pop()
	if job == nil {
		return nil
	}
	c.curConcurrency++
	return job
}

// 循环执行任务
func (c *workerQueue) do(job *asyncJob) {
	for job != nil {
		err := job.execute(job.socket, job.frame)
		job.socket.emitError(err)
		job = c.getJob(nil, -1)
	}
}

// Push 追加任务, 有资源空闲的话会立即执行
func (c *workerQueue) Push(job *asyncJob) {
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

type heap struct {
	data   []*asyncJob
	serial int
}

func (c *heap) next() int {
	c.serial++
	return c.serial
}

func (c *heap) less(i, j int) bool {
	return c.data[i].serial < c.data[j].serial
}

func (c *heap) Len() int {
	return len(c.data)
}

func (c *heap) swap(i, j int) {
	c.data[i], c.data[j] = c.data[j], c.data[i]
}

func (c *heap) Push(v *asyncJob) {
	if v.serial == 0 {
		v.serial = c.next()
	}
	c.data = append(c.data, v)
	c.up(c.Len() - 1)
}

func (c *heap) up(i int) {
	var j = (i - 1) / 2
	if i >= 1 && c.less(i, j) {
		c.swap(i, j)
		c.up(j)
	}
}

func (c *heap) Pop() *asyncJob {
	n := c.Len()
	switch n {
	case 0:
		return nil
	case 1:
		v := c.data[0]
		c.data = c.data[:0]
		return v
	default:
		v := c.data[0]
		c.data[0] = c.data[n-1]
		c.data = c.data[:n-1]
		c.down(0, n-1)
		return v
	}
}

func (c *heap) down(i, n int) {
	var j = 2*i + 1
	var k = 2*i + 2
	var x = -1
	if j < n {
		x = j
	}
	if k < n && c.less(k, j) {
		x = k
	}
	if x != -1 && c.less(x, i) {
		c.swap(i, x)
		c.down(x, n)
	}
}
