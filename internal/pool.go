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
		return NewBuffer(make([]byte, 0, Bv7))
	}
	p.p1.New = func() interface{} {
		return NewBuffer(make([]byte, 0, Bv10))
	}
	p.p2.New = func() interface{} {
		return NewBuffer(make([]byte, 0, Bv12))
	}
	p.p3.New = func() interface{} {
		return NewBuffer(nil)
	}
	return p
}

func (p *BufferPool) Put(b *Buffer) {
	n := b.Cap()
	if n <= Bv7 {
		p.p0.Put(b)
		return
	}
	if n <= Bv10 {
		p.p1.Put(b)
		return
	}
	if n <= Bv12 {
		p.p2.Put(b)
		return
	}
	if n <= Bv16 {
		p.p3.Put(b)
		return
	}
}

func (p *BufferPool) Get(n int) *Buffer {
	if n <= Bv7 {
		buf := p.p0.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	if n <= Bv10 {
		buf := p.p1.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	if n <= Bv12 {
		buf := p.p2.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	if n <= Bv16 {
		buf := p.p3.Get().(*Buffer)
		buf.Reset()
		return buf
	}
	return NewBuffer(nil)
}
