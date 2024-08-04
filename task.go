package gws

import (
	"sync"

	"github.com/lxzan/gws/internal"
)

type (
	// workerQueue 代表一个任务队列
	// workerQueue represents a task queue
	workerQueue struct {
		// mu 是一个互斥锁，用于保护对队列的并发访问
		// mu is a mutex to protect concurrent access to the queue
		mu sync.Mutex

		// q 是一个双端队列，用于存储异步任务
		// q is a double-ended queue to store asynchronous jobs
		q internal.Deque[asyncJob]

		// maxConcurrency 是最大并发数
		// maxConcurrency is the maximum concurrency
		maxConcurrency int32

		// curConcurrency 是当前并发数
		// curConcurrency is the current concurrency
		curConcurrency int32
	}

	// asyncJob 代表一个异步任务
	// asyncJob represents an asynchronous job
	asyncJob func()
)

// newWorkerQueue 创建一个任务队列
// newWorkerQueue creates a task queue
func newWorkerQueue(maxConcurrency int32) *workerQueue {
	c := &workerQueue{
		// 初始化互斥锁
		// Initialize the mutex
		mu: sync.Mutex{},

		// 设置最大并发数
		// Set the maximum concurrency
		maxConcurrency: maxConcurrency,

		// 初始化当前并发数为 0
		// Initialize the current concurrency to 0
		curConcurrency: 0,
	}

	// 返回初始化的任务队列
	// Return the initialized task queue
	return c
}

// 获取一个任务
// getJob retrieves a job from the worker queue
func (c *workerQueue) getJob(newJob asyncJob, delta int32) asyncJob {
	// 加锁以确保线程安全
	// Lock to ensure thread safety
	c.mu.Lock()
	// 在函数结束时解锁
	// Unlock at the end of the function
	defer c.mu.Unlock()

	// 如果有新任务，将其添加到队列中
	// If there is a new job, add it to the queue
	if newJob != nil {
		c.q.PushBack(newJob)
	}

	// 更新当前并发数
	// Update the current concurrency count
	c.curConcurrency += delta

	// 如果当前并发数达到或超过最大并发数，返回 nil
	// If the current concurrency count reaches or exceeds the maximum concurrency, return nil
	if c.curConcurrency >= c.maxConcurrency {
		return nil
	}

	// 从队列中取出一个任务
	// Retrieve a job from the queue
	var job = c.q.PopFront()

	// 如果队列为空，返回 nil
	// If the queue is empty, return nil
	if job == nil {
		return nil
	}

	// 增加当前并发数
	// Increment the current concurrency count
	c.curConcurrency++

	// 返回取出的任务
	// Return the retrieved job
	return job
}

// 循环执行任务
// do continuously executes jobs in the worker queue
func (c *workerQueue) do(job asyncJob) {
	// 当任务不为空时，循环执行任务
	// Loop to execute jobs as long as the job is not nil
	for job != nil {
		// 执行当前任务
		// Execute the current job
		job()
		// 获取下一个任务并减少当前并发数
		// Get the next job and decrement the current concurrency count
		job = c.getJob(nil, -1)
	}
}

// Push 追加任务, 有资源空闲的话会立即执行
// Push adds a job to the queue and executes it immediately if resources are available
func (c *workerQueue) Push(job asyncJob) {
	// 获取下一个任务，如果有资源空闲的话
	// Get the next job if resources are available
	if nextJob := c.getJob(job, 0); nextJob != nil {
		// 启动一个新的 goroutine 来执行任务
		// Start a new goroutine to execute the job
		go c.do(nextJob)
	}
}

// 定义一个名为 channel 的类型，底层类型为 struct{} 的通道
// Define a type named channel, which is a channel of struct{}
type channel chan struct{}

// add 方法向通道发送一个空的 struct{}，表示增加一个任务
// The add method sends an empty struct{} to the channel, indicating the addition of a task
func (c channel) add() { c <- struct{}{} }

// done 方法从通道接收一个空的 struct{}，表示完成一个任务
// The done method receives an empty struct{} from the channel, indicating the completion of a task
func (c channel) done() { <-c }

// Go 方法接收一个消息和一个函数，启动一个新的 goroutine 来执行该函数
// The Go method receives a message and a function, and starts a new goroutine to execute the function
func (c channel) Go(m *Message, f func(*Message) error) error {
	// 增加一个任务
	// Add a task
	c.add()

	// 启动一个新的 goroutine 来执行函数 f
	// Start a new goroutine to execute the function f
	go func() {
		// 执行函数 f，并忽略其返回值
		// Execute the function f and ignore its return value
		_ = f(m)
		// 完成一个任务
		// Complete a task
		c.done()
	}()

	// 返回 nil 表示成功
	// Return nil to indicate success
	return nil
}
