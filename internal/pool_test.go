package internal

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferPool(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool()

	for i := 0; i < 10; i++ {
		var n = AlphabetNumeric.Intn(126)
		var buf = pool.Get(n)
		as.Equal(128, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(500)
		as.Equal(Lv2, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(2000)
		as.Equal(Lv3, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(5000)
		as.Equal(Lv5, buf.Cap())
		as.Equal(0, buf.Len())
	}

	{
		pool.Put(bytes.NewBuffer(make([]byte, 2)))
		b := pool.Get(120)
		as.GreaterOrEqual(b.Cap(), 120)
	}
	{
		pool.Put(bytes.NewBuffer(make([]byte, 2000)))
		b := pool.Get(3000)
		as.GreaterOrEqual(b.Cap(), 3000)
	}

	pool.Put(nil)
	buffer := pool.Get(256 * 1024)
	as.GreaterOrEqual(buffer.Cap(), 256*1024)
}

func TestPool(t *testing.T) {
	var p = NewPool(func() int {
		return 0
	})
	assert.Equal(t, 0, p.Get())
	p.Put(1)
}

func TestBufferPool_GetIndex(t *testing.T) {
	var p = NewBufferPool()
	assert.Equal(t, p.getIndex(200*1024), 0)

	assert.Equal(t, p.getIndex(0), 1)
	assert.Equal(t, p.getIndex(1), 1)
	assert.Equal(t, p.getIndex(10), 1)
	assert.Equal(t, p.getIndex(100), 1)
	assert.Equal(t, p.getIndex(128), 1)

	assert.Equal(t, p.getIndex(200), 2)
	assert.Equal(t, p.getIndex(1000), 2)
	assert.Equal(t, p.getIndex(500), 2)
	assert.Equal(t, p.getIndex(1024), 2)

	assert.Equal(t, p.getIndex(2*1024), 3)
	assert.Equal(t, p.getIndex(2000), 3)
	assert.Equal(t, p.getIndex(1025), 3)

	assert.Equal(t, p.getIndex(4*1024), 4)
	assert.Equal(t, p.getIndex(3000), 4)
	assert.Equal(t, p.getIndex(2*1024+1), 4)

	assert.Equal(t, p.getIndex(8*1024), 5)
	assert.Equal(t, p.getIndex(5000), 5)
	assert.Equal(t, p.getIndex(4*1024+1), 5)

	assert.Equal(t, p.getIndex(16*1024), 6)
	assert.Equal(t, p.getIndex(10000), 6)
	assert.Equal(t, p.getIndex(8*1024+1), 6)

	assert.Equal(t, p.getIndex(32*1024), 7)
	assert.Equal(t, p.getIndex(20000), 7)
	assert.Equal(t, p.getIndex(16*1024+1), 7)

	assert.Equal(t, p.getIndex(64*1024), 8)
	assert.Equal(t, p.getIndex(40000), 8)
	assert.Equal(t, p.getIndex(32*1024+1), 8)

	assert.Equal(t, p.getIndex(128*1024), 9)
	assert.Equal(t, p.getIndex(100000), 9)
	assert.Equal(t, p.getIndex(64*1024+1), 9)
}

func BenchmarkPool_GetIndex(b *testing.B) {
	var p = NewBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000000; j++ {
			p.getIndex(uint32(j))
		}
	}
}
