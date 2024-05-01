package internal

import (
	"bytes"
	"sync"
)

type BufferPool struct {
	begin      uint32
	shards     []*sync.Pool
	size2index map[int]int
}

// NewBufferPool Creating a memory pool
// Left, right indicate the interval range of the memory pool, they will be transformed into pow(2,n)。
// Below left, Get method will return at least left bytes; above right, Put method will not reclaim the buffer.
func NewBufferPool(left, right uint32) *BufferPool {
	var begin, end = int(binaryCeil(left)), int(binaryCeil(right))
	var p = &BufferPool{begin: uint32(begin), size2index: map[int]int{}}
	for i, j := begin, 0; i <= end; i *= 2 {
		capacity := i
		pool := &sync.Pool{New: func() any { return bytes.NewBuffer(make([]byte, 0, capacity)) }}
		p.shards = append(p.shards, pool)
		p.size2index[i] = j
		j++
	}
	return p
}

// Put Return buffer to memory pool
func (p *BufferPool) Put(b *bytes.Buffer) {
	if b != nil {
		if index, ok := p.size2index[b.Cap()]; ok {
			p.shards[index].Put(b)
		}
	}
}

// Get Fetch a buffer from the memory pool, of at least n bytes
func (p *BufferPool) Get(n int) *bytes.Buffer {
	var size = int(Max(binaryCeil(uint32(n)), p.begin))
	if index, ok := p.size2index[size]; ok {
		b := p.shards[index].Get().(*bytes.Buffer)
		if b.Cap() < size {
			b.Grow(size)
		}
		b.Reset()
		return b
	}
	return bytes.NewBuffer(make([]byte, 0, n))
}

func binaryCeil(v uint32) uint32 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}

func NewPool[T any](f func() T) *Pool[T] {
	return &Pool[T]{p: sync.Pool{New: func() any { return f() }}}
}

type Pool[T any] struct {
	p sync.Pool
}

func (c *Pool[T]) Put(v T) { c.p.Put(v) }

func (c *Pool[T]) Get() T { return c.p.Get().(T) }
