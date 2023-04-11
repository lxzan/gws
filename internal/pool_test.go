package internal

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBufferPool(t *testing.T) {
	var as = assert.New(t)
	var pool = NewBufferPool()

	pool.Put(bytes.NewBuffer(AlphabetNumeric.Generate(64)))
	as.GreaterOrEqual(pool.Get(72).Cap(), 72)

	{
		pool.Put(bytes.NewBuffer(AlphabetNumeric.Generate(128)))
		pool.Put(bytes.NewBuffer(AlphabetNumeric.Generate(Lv2)))
		pool.Put(bytes.NewBuffer(AlphabetNumeric.Generate(Lv3)))
		pool.Put(bytes.NewBuffer(AlphabetNumeric.Generate(Lv4)))
	}

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

	pool.Put(nil)
	pool.Put(NewBufferWithCap(0))
	pool.Get(17 * 1024)
}
