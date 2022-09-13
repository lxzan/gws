package internal

import (
	"bytes"
	"sync"
)

type BufferPool struct {
	p1 sync.Pool
	p2 sync.Pool
	p3 sync.Pool
}

func NewBufferPool() *BufferPool {
	var p = &BufferPool{
		p1: sync.Pool{},
		p2: sync.Pool{},
		p3: sync.Pool{},
	}
	p.p1.New = func() interface{} {
		return bytes.NewBuffer(nil)
	}
	p.p2.New = func() interface{} {
		return bytes.NewBuffer(nil)
	}
	p.p3.New = func() interface{} {
		return bytes.NewBuffer(nil)
	}
	return p
}

func (p *BufferPool) Put(b *bytes.Buffer) {
	b.Reset()

	n := b.Cap()
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

func (p *BufferPool) Get(n int) *bytes.Buffer {
	if n <= Bv10 {
		return p.p1.Get().(*bytes.Buffer)
	}
	if n <= Bv12 {
		return p.p2.Get().(*bytes.Buffer)
	}
	if n <= Bv16 {
		return p.p3.Get().(*bytes.Buffer)
	}
	return bytes.NewBuffer(nil)
}
