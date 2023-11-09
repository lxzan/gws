package internal

import (
	"bytes"
	"sync"
)

const (
	poolSize = 10

	Lv1 = 128
	Lv2 = 1024
	Lv3 = 2 * 1024
	Lv4 = 4 * 1024
	Lv5 = 8 * 1024
	Lv6 = 16 * 1024
	Lv7 = 32 * 1024
	Lv8 = 64 * 1024
	Lv9 = 128 * 1024
)

type BufferPool struct {
	pools  []*sync.Pool
	limits []int
}

func NewBufferPool() *BufferPool {
	var p = &BufferPool{
		pools:  make([]*sync.Pool, poolSize),
		limits: []int{0, Lv1, Lv2, Lv3, Lv4, Lv5, Lv6, Lv7, Lv8, Lv9},
	}
	for i := 1; i < poolSize; i++ {
		var capacity = p.limits[i]
		p.pools[i] = &sync.Pool{New: func() any {
			return bytes.NewBuffer(make([]byte, 0, capacity))
		}}
	}
	return p
}

func (p *BufferPool) Put(b *bytes.Buffer) {
	if b == nil || b.Cap() == 0 {
		return
	}
	if index := p.getIndex(uint32(b.Cap())); index > 0 {
		p.pools[index].Put(b)
	}
}

func (p *BufferPool) Get(n int) *bytes.Buffer {
	var index = p.getIndex(uint32(n))
	if index == 0 {
		return bytes.NewBuffer(make([]byte, 0, n))
	}

	b := p.pools[index].Get().(*bytes.Buffer)
	if b.Cap() < n {
		b.Grow(p.limits[index])
	}
	b.Reset()
	return b
}

func (p *BufferPool) getIndex(v uint32) int {
	if v > Lv9 {
		return 0
	}
	if v <= 128 {
		return 1
	}

	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++

	switch v {
	case Lv3:
		return 3
	case Lv4:
		return 4
	case Lv5:
		return 5
	case Lv6:
		return 6
	case Lv7:
		return 7
	case Lv8:
		return 8
	case Lv9:
		return 9
	default:
		return 2
	}
}

func NewPool[T any](f func() T) *Pool[T] {
	return &Pool[T]{p: sync.Pool{New: func() any { return f() }}}
}

type Pool[T any] struct {
	p sync.Pool
}

func (c *Pool[T]) Put(v T) { c.p.Put(v) }

func (c *Pool[T]) Get() T { return c.p.Get().(T) }
