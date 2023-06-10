package internal

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
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
		as.Equal(Lv4, buf.Cap())
		as.Equal(0, buf.Len())
	}

	{
		pool.Put(bytes.NewBuffer(make([]byte, 2)), 2)
		b := pool.Get(120)
		as.GreaterOrEqual(b.Cap(), 120)
	}
	{
		pool.Put(bytes.NewBuffer(make([]byte, 2000)), 2000)
		b := pool.Get(3000)
		as.GreaterOrEqual(b.Cap(), 3000)
	}

	pool.Put(nil, 0)
	pool.Put(NewBufferWithCap(0), 0)
	as.GreaterOrEqual(pool.Get(128*1024).Cap(), 128*1024)
}

func TestBufferPool_GetvCap(t *testing.T) {
	var as = assert.New(t)
	var p = NewBufferPool()
	as.Equal(Lv2, p.GetvCap(512))
	as.Equal(Lv3, p.GetvCap(3*1024))
	as.Equal(Lv4, p.GetvCap(8*1024))
	as.Equal(Lv4, p.GetvCap(256*1024))
}
