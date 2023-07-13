package internal

import (
	"bytes"
	"sync"
)

const poolSize = 9

type BufferPool struct {
	pools  [poolSize]*sync.Pool
	limits [poolSize]int
}

func NewBufferPool() *BufferPool {
	var p BufferPool
	p.limits = [poolSize]int{0, Lv1, Lv2, Lv3, Lv4, Lv5, Lv6, Lv7, Lv8}
	for i := 1; i < poolSize; i++ {
		var capacity = p.limits[i]
		p.pools[i] = &sync.Pool{New: func() any {
			return bytes.NewBuffer(make([]byte, 0, capacity))
		}}
	}
	return &p
}

func (p *BufferPool) Put(b *bytes.Buffer, index int) {
	if b == nil || index == 0 {
		return
	}
	if b.Cap() <= 2*p.limits[index] {
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
