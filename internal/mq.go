package internal

import "sync"

type Iterator struct {
	next *Iterator
	Data interface{}
}

func NewQueue(concurrency int64) *Queue {
	return &Queue{
		Mutex:       sync.Mutex{},
		length:      0,
		concurrency: concurrency,
		running:     0,
		head:        nil,
		tail:        nil,
	}
}

type Queue struct {
	sync.Mutex
	length      int
	concurrency int64
	running     int64
	head        *Iterator
	tail        *Iterator
}

func (c *Queue) Clear() {
	c.head = nil
	c.tail = nil
	c.length = 0
}

func (c *Queue) Len() int {
	c.Lock()
	defer c.Unlock()

	return c.length
}

func (c *Queue) Push(v interface{}) {
	c.Lock()
	defer c.Unlock()

	var ele = &Iterator{Data: v}
	if c.length > 0 {
		c.tail.next = ele
		c.tail = ele
	} else {
		c.head = ele
		c.tail = ele
	}
	c.length++
}

func (c *Queue) Done() {
	c.Lock()
	c.running--
	c.Unlock()
}

func (c *Queue) Pop() *Iterator {
	c.Lock()
	defer c.Unlock()

	if c.running >= c.concurrency {
		return nil
	}

	var ele = c.doPop()
	if ele != nil {
		c.running++
	}
	return ele
}

func (c *Queue) doPop() *Iterator {
	if c.length == 0 {
		return nil
	}
	var result = c.head
	c.head = c.head.next
	c.length--
	if c.length == 0 {
		c.tail = nil
	}
	return result
}

func NewTokenBucket(capacity int) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		mu:       sync.Mutex{},
		bucket:   make([]uint8, capacity, capacity),
	}
}

type TokenBucket struct {
	capacity int
	mu       sync.Mutex
	bucket   []uint8
}

func (c *TokenBucket) Reset() {
	c.mu.Lock()
	c.bucket = make([]uint8, c.capacity, c.capacity)
	c.mu.Unlock()
}

func (c *TokenBucket) Len() int {
	c.mu.Lock()
	n := len(c.bucket)
	c.mu.Unlock()
	return n
}

func (c *TokenBucket) Push() {
	c.mu.Lock()
	c.bucket = append(c.bucket, 1)
	c.mu.Unlock()
}

func (c *TokenBucket) Pop() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	n := len(c.bucket)
	if n > 0 {
		c.bucket = c.bucket[:n-1]
		return 1
	}
	return 0
}
