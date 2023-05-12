package gws

import (
	"github.com/lxzan/gws/internal"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferPool(t *testing.T) {
	var as = assert.New(t)
	var pool = newBufferPool()

	pool.Put(nil)
	pool.Put(newBuffer(nil, 0))
	pool.Put(newBuffer(internal.AlphabetNumeric.Generate(64), 1))
	as.GreaterOrEqual(pool.Get(72).Cap(), 72)

	{
		pool.Put(newBuffer(internal.AlphabetNumeric.Generate(128), 1))
		pool.Put(newBuffer(internal.AlphabetNumeric.Generate(internal.Lv2), 2))
		pool.Put(newBuffer(internal.AlphabetNumeric.Generate(internal.Lv3), 3))
		pool.Put(newBuffer(internal.AlphabetNumeric.Generate(internal.Lv4), 4))
	}

	for i := 0; i < 10; i++ {
		var n = internal.AlphabetNumeric.Intn(126)
		var buf = pool.Get(n)
		as.Equal(128, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(500)
		as.Equal(internal.Lv2, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(2000)
		as.Equal(internal.Lv3, buf.Cap())
		as.Equal(0, buf.Len())
	}
	for i := 0; i < 10; i++ {
		var buf = pool.Get(5000)
		as.Equal(internal.Lv4, buf.Cap())
		as.Equal(0, buf.Len())
	}

	pool.Put(nil)
	//pool.Put(internal.NewBufferWithCap(0))
	pool.Get(17 * 1024)
	pool.Put(newBuffer(make([]byte, 32*1024), 3))
	as.GreaterOrEqual(pool.Get(128*1024).Cap(), 128*1024)
}
