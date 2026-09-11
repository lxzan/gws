package internal

import (
	"bytes"
	"math/bits"
	"sync"
)

type BufferPool struct {
	begin    int
	end      int
	minShift int
	maxShift int
	shards   [32]sync.Pool
}

// NewBufferPool 创建一个内存池
// creates a memory pool
// left 和 right 表示内存池的区间范围，它们将被转换为 2 的 n 次幂
// left and right indicate the interval range of the memory pool, they will be transformed into pow(2, n)
// 小于 left 的情况下，Get 方法将返回至少 left 字节的缓冲区；大于 right 的情况下，Put 方法不会回收缓冲区
// Below left, the Get method will return at least left bytes; above right, the Put method will not reclaim the buffer
func NewBufferPool(left, right uint32) *BufferPool {
	var begin, end = int(binaryCeil(left)), int(binaryCeil(right))
	minShift := bits.Len32(uint32(begin - 1))
	maxShift := bits.Len32(uint32(end - 1))
	var p = &BufferPool{
		begin:    begin,
		end:      end,
		minShift: minShift,
		maxShift: maxShift,
	}
	for i := minShift; i <= maxShift; i++ {
		capacity := 1 << i
		p.shards[i] = sync.Pool{
			New: func() any { return bytes.NewBuffer(make([]byte, 0, capacity)) },
		}
	}
	return p
}

// Put 将缓冲区放回到内存池
// returns the buffer to the memory pool
func (p *BufferPool) Put(b *bytes.Buffer) {
	if b == nil {
		return
	}
	c := b.Cap()
	if c < p.begin || c > p.end || c&(c-1) != 0 {
		return
	}
	shift := bits.TrailingZeros32(uint32(c))
	p.shards[shift].Put(b)
}

// Get 从内存池中获取一个至少 n 字节的缓冲区
// fetches a buffer from the memory pool, of at least n bytes
func (p *BufferPool) Get(n int) *bytes.Buffer {
	if n <= p.begin {
		b := p.shards[p.minShift].Get().(*bytes.Buffer)
		if b.Cap() < p.begin {
			b.Grow(p.begin)
		}
		b.Reset()
		return b
	}
	if n > p.end {
		return bytes.NewBuffer(make([]byte, 0, n))
	}
	shift := int(bits.Len32(uint32(n - 1)))
	if shift <= p.maxShift {
		b := p.shards[shift].Get().(*bytes.Buffer)
		targetCap := 1 << shift
		if b.Cap() < targetCap {
			b.Grow(targetCap)
		}
		b.Reset()
		return b
	}
	return bytes.NewBuffer(make([]byte, 0, n))
}

// binaryCeil 将给定的 uint32 值向上取整到最近的 2 的幂
// rounds up the given uint32 value to the nearest power of 2
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

// NewPool 创建一个新的泛型内存池
// creates a new generic pool
func NewPool[T any](f func() T) *Pool[T] {
	return &Pool[T]{p: sync.Pool{New: func() any { return f() }}}
}

// Pool 泛型内存池
// generic pool
type Pool[T any] struct {
	p sync.Pool // 内嵌的 sync.Pool
}

// Put 将一个值放入池中
// puts a value into the pool
func (c *Pool[T]) Put(v T) {
	c.p.Put(v)
}

// Get 从池中获取一个值
// gets a value from the pool
func (c *Pool[T]) Get() T {
	return c.p.Get().(T)
}
