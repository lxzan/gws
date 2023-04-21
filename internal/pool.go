package internal

import (
	"bytes"
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
		return bytes.NewBuffer(make([]byte, 0, Lv1))
	}
	p.p1.New = func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, Lv2))
	}
	p.p2.New = func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, Lv3))
	}
	p.p3.New = func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, Lv4))
	}
	return p
}

func (p *BufferPool) Put(b *bytes.Buffer) {
	if b == nil || b.Cap() == 0 {
		return
	}

	b.Reset()
	n := b.Cap()
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

func (p *BufferPool) Get(n int) *bytes.Buffer {
	buf := p.doGet(n)
	if buf.Cap() < n {
		buf.Grow(n)
	}
	return buf
}

func (p *BufferPool) doGet(n int) *bytes.Buffer {
	if n <= Lv1 {
		return p.p0.Get().(*bytes.Buffer)
	}
	if n <= Lv2 {
		return p.p1.Get().(*bytes.Buffer)
	}
	if n <= Lv3 {
		return p.p2.Get().(*bytes.Buffer)
	}
	if n <= Lv4 {
		return p.p3.Get().(*bytes.Buffer)
	}
	return bytes.NewBuffer(make([]byte, 0, n))
}
