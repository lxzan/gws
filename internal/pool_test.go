package internal

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferPool(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool(128, 128*1024)

	for i := 0; i < 10; i++ {
		var n = AlphabetNumeric.Intn(126)
		var buf = pool.Get(n)
		as.Equal(128, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(500)
		as.Equal(512, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(2000)
		as.Equal(2048, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
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

func TestPool(t *testing.T) {
	var p = NewPool(func() int {
		return 0
	})
	assert.Equal(t, 0, p.Get())
	p.Put(1)
}
