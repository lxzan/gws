package internal

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferPool(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool(128, 128*1024)

	pool.Put(bytes.NewBuffer(AlphabetNumeric.Generate(128)))
	for range 10 {
		var n = AlphabetNumeric.Intn(126)
		var buf = pool.Get(n)
		as.Equal(128, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for range 10 {
		var buf = pool.Get(500)
		as.Equal(512, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for range 10 {
		var buf = pool.Get(2000)
		as.Equal(2048, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for range 10 {
		var buf = pool.Get(5000)
		as.Equal(8192, buf.Cap())
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

func TestBufferPoolGetBoundary(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool(128, 8192)

	var cases = []struct {
		n       int
		wantCap int
	}{
		{n: 0, wantCap: 128},
		{n: 1, wantCap: 128},
		{n: 127, wantCap: 128},
		{n: 128, wantCap: 128},
		{n: 129, wantCap: 256},
		{n: 500, wantCap: 512},
		{n: 8191, wantCap: 8192},
		{n: 8192, wantCap: 8192},
	}
	for _, item := range cases {
		var buf = pool.Get(item.n)
		as.Equal(item.wantCap, buf.Cap(), "n=%d", item.n)
		as.Equal(0, buf.Len())
	}

	for _, n := range []int{8193, 65536} {
		var buf = pool.Get(n)
		as.GreaterOrEqual(buf.Cap(), n)
		as.Equal(0, buf.Len())
	}
}

func TestBufferPoolPutBoundary(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool(128, 8192)

	pool.Put(nil)
	pool.Put(bytes.NewBuffer(make([]byte, 0, 64)))
	pool.Put(bytes.NewBuffer(make([]byte, 0, 300)))
	pool.Put(bytes.NewBuffer(make([]byte, 0, 16384)))
	pool.Put(bytes.NewBuffer(make([]byte, 0, 128)))
	pool.Put(bytes.NewBuffer(make([]byte, 0, 8192)))

	var buf = pool.Get(128)
	as.Equal(128, buf.Cap())
	as.Equal(0, buf.Len())
	buf = pool.Get(8192)
	as.Equal(8192, buf.Cap())
	as.Equal(0, buf.Len())
}

func TestBufferPoolRoundTrip(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool(128, 8192)

	for size := 128; size <= 8192; size *= 2 {
		var buf = bytes.NewBuffer(make([]byte, 0, size))
		buf.WriteString("payload")
		pool.Put(buf)

		var got = pool.Get(size)
		as.Equal(size, got.Cap())
		as.Equal(0, got.Len())
	}
}

func TestBufferPoolZeroValue(t *testing.T) {
	var as = assert.New(t)
	var pool BufferPool

	var buf = pool.Get(256)
	as.NotNil(buf)
	as.GreaterOrEqual(buf.Cap(), 256)
	as.Equal(0, buf.Len())
	pool.Put(buf)
	pool.Put(nil)
}

func TestBufferPoolConcurrent(t *testing.T) {
	var pool = NewBufferPool(128, 8192)
	var sizes = []int{1, 100, 128, 255, 512, 2048, 8192, 10000}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				for _, n := range sizes {
					var buf = pool.Get(n)
					buf.WriteByte('x')
					pool.Put(buf)
				}
			}
		}()
	}
	wg.Wait()
}

func TestBufferPoolShardLayout(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool(128, 8192)

	as.Len(pool.shards, 7)
	pool.shards[0].Put(bytes.NewBuffer(make([]byte, 0, 128)))
	as.Equal(128, pool.Get(128).Cap())
	pool.shards[6].Put(bytes.NewBuffer(make([]byte, 0, 8192)))
	as.Equal(8192, pool.Get(8192).Cap())
}

func TestPool(t *testing.T) {
	var p = NewPool(func() int {
		return 0
	})
	assert.Equal(t, 0, p.Get())
	p.Put(1)
}

func TestPool_Get(t *testing.T) {
	var p = NewBufferPool(128, 1024*128)
	p.shards[0].Put(bytes.NewBuffer(AlphabetNumeric.Generate(120)))
	var buf = p.Get(128)
	assert.GreaterOrEqual(t, buf.Cap(), 128)
}
