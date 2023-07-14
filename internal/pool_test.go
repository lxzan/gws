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
		var buf, index = pool.Get(n)
		as.Equal(128, buf.Cap())
		as.Equal(0, buf.Len())
		as.Equal(index, 1)
	}
	for i := 0; i < 10; i++ {
		var buf, index = pool.Get(500)
		as.Equal(Lv2, buf.Cap())
		as.Equal(0, buf.Len())
		as.Equal(index, 2)
	}
	for i := 0; i < 10; i++ {
		var buf, index = pool.Get(2000)
		as.Equal(Lv3, buf.Cap())
		as.Equal(0, buf.Len())
		as.Equal(index, 3)
	}
	for i := 0; i < 10; i++ {
		var buf, index = pool.Get(5000)
		as.Equal(Lv5, buf.Cap())
		as.Equal(0, buf.Len())
		as.Equal(index, 5)
	}

	{
		pool.Put(bytes.NewBuffer(make([]byte, 2)), 2)
		b, index := pool.Get(120)
		as.GreaterOrEqual(b.Cap(), 120)
		as.Equal(index, 1)
	}
	{
		pool.Put(bytes.NewBuffer(make([]byte, 2000)), 4)
		b, index := pool.Get(3000)
		as.GreaterOrEqual(b.Cap(), 3000)
		as.Equal(index, 4)
	}

	pool.Put(nil, 0)
	pool.Put(NewBufferWithCap(0), 0)
	buffer, _ := pool.Get(128 * 1024)
	as.GreaterOrEqual(buffer.Cap(), 128*1024)
}
