package internal

import (
	"bytes"
	"sync"
)

type BufferPool struct {
	pools  [6]*sync.Pool
	limits [6]int
}

func NewBufferPool() *BufferPool {
	var p BufferPool
	p.limits = [6]int{0, Lv1, Lv2, Lv3, Lv4, Lv5}
	for i := 1; i < 6; i++ {
		var capacity = p.limits[i]
		p.pools[i] = &sync.Pool{New: func() any {
			return bytes.NewBuffer(make([]byte, 0, capacity))
		}}
	}
	return &p
}

func (p *BufferPool) Put(b *bytes.Buffer, n int) {
	if b == nil || n == 0 {
		return
	}
	for i := 1; i < 6; i++ {
		if n <= p.limits[i] {
			if b.Cap() <= 4*p.limits[i] {
				p.pools[i].Put(b)
			}
			return
		}
	}
}

func (p *BufferPool) Get(n int) *bytes.Buffer {
	for i := 1; i < 6; i++ {
		if n <= p.limits[i] {
			b := p.pools[i].Get().(*bytes.Buffer)
			if b.Cap() < n {
				b.Grow(p.limits[i])
			}
			b.Reset()
			return b
		}
	}
	return bytes.NewBuffer(make([]byte, 0, n))
}
