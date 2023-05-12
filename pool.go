package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"sync"
)

type Buffer struct {
	pIndex int
	*bytes.Buffer
}

func newBuffer(b []byte, pIndex int) *Buffer {
	return &Buffer{
		pIndex: pIndex,
		Buffer: bytes.NewBuffer(b),
	}
}

type bufferPool struct {
	pools  [6]*sync.Pool
	limits [6]int
}

func newBufferPool() *bufferPool {
	var p bufferPool
	p.limits = [6]int{0, internal.Lv1, internal.Lv2, internal.Lv3, internal.Lv4, internal.Lv5}
	for i := 1; i < 6; i++ {
		var capacity = p.limits[i]
		p.pools[i] = &sync.Pool{New: func() any {
			return newBuffer(make([]byte, 0, capacity), i)
		}}
	}
	return &p
}

func (p *bufferPool) Put(b *Buffer) {
	if b == nil {
		return
	}
	if capacity := b.Cap(); b.pIndex <= 0 || capacity == 0 || capacity > 2*p.limits[b.pIndex] {
		return
	}
	p.pools[b.pIndex].Put(b)
}

func (p *bufferPool) Get(n int) *Buffer {
	buf := p.doGet(n)
	if buf.Cap() < n {
		buf.Grow(n)
	}
	buf.Reset()
	return buf
}

func (p *bufferPool) doGet(n int) *Buffer {
	for i := 1; i < 6; i++ {
		if n <= p.limits[i] {
			b := p.pools[i].Get().(*Buffer)
			b.pIndex = i
			return b
		}
	}
	return newBuffer(make([]byte, 0, n), -1)
}
