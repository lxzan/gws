package internal

import (
	"sync"
)

type BufferPool struct {
	p0 sync.Pool
	p1 sync.Pool
	p2 sync.Pool
	p3 sync.Pool
}

func NewBufferPool() *BufferPool {
	var p = &BufferPool{
		p0: sync.Pool{},
		p1: sync.Pool{},
		p2: sync.Pool{},
		p3: sync.Pool{},
	}
	p.p0.New = func() interface{} {
		return NewBuffer(make([]byte, 0, Lv1))
	}
	p.p1.New = func() interface{} {
		return NewBuffer(make([]byte, 0, Lv2))
	}
	p.p2.New = func() interface{} {
		return NewBuffer(make([]byte, 0, Lv3))
	}
	p.p3.New = func() interface{} {
		return NewBuffer(nil)
	}
	return p
}

func (p *BufferPool) Put(b *Buffer) {
	n := b.Cap()
	if n == 0 || n > Lv4 {
		return
	}

	b.Reset()
	if n <= Lv1 {
		p.p0.Put(b)
		return
	}
	if n <= Lv2 {
		p.p1.Put(b)
		return
	}
	if n <= Lv3 {
		p.p2.Put(b)
		return
	}
	if n <= Lv4 {
		p.p3.Put(b)
		return
	}
}

func (p *BufferPool) Get(n int) *Buffer {
	if n <= Lv1 {
		buf := p.p0.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	if n <= Lv2 {
		buf := p.p1.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	if n <= Lv3 {
		buf := p.p2.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	if n <= Lv4 {
		buf := p.p3.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	return NewBuffer(make([]byte, 0, Lv4))
}
