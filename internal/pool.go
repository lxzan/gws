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
	pools  [poolSize]*sync.Pool
	limits [poolSize]int
}

func NewBufferPool() *BufferPool {
	var p BufferPool
	p.limits = [poolSize]int{0, Lv1, Lv2, Lv3, Lv4, Lv5, Lv6, Lv7, Lv8, Lv9}
	for i := 1; i < poolSize; i++ {
		var capacity = p.limits[i]
		p.pools[i] = &sync.Pool{New: func() any {
			return bytes.NewBuffer(make([]byte, 0, capacity))
		}}
	}
	return &p
}

func (p *BufferPool) Put(b *bytes.Buffer, index int) {
	if index == 0 || b == nil {
		return
	}
	if b.Cap() <= 5*p.limits[index] {
		p.pools[index].Put(b)
	}
}

func (p *BufferPool) Get(n int) (*bytes.Buffer, int) {
	for i := 1; i < poolSize; i++ {
		if n <= p.limits[i] {
			b := p.pools[i].Get().(*bytes.Buffer)
			if b.Cap() < n {
				b.Grow(p.limits[i])
			}
			b.Reset()
			return b, i
		}
	}
	return bytes.NewBuffer(make([]byte, 0, n)), 0
}
