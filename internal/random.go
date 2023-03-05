package internal

import (
	"math/rand"
	"sync"
	"time"
)

type RandomString struct {
	mu     sync.Mutex
	r      *rand.Rand
	layout string
}

var (
	AlphabetNumeric = &RandomString{
		layout: "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mu:     sync.Mutex{},
	}

	Numeric = &RandomString{
		layout: "0123456789",
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mu:     sync.Mutex{},
	}
)

func (c *RandomString) Generate(n int) []byte {
	c.mu.Lock()
	var b = make([]byte, n, n)
	var length = len(c.layout)
	for i := 0; i < n; i++ {
		var idx = c.r.Intn(length)
		b[i] = c.layout[idx]
	}
	c.mu.Unlock()
	return b
}

func (c *RandomString) Intn(n int) int {
	c.mu.Lock()
	x := c.r.Intn(n)
	c.mu.Unlock()
	return x
}

func (c *RandomString) Uint32() uint32 {
	c.mu.Lock()
	x := c.r.Uint32()
	c.mu.Unlock()
	return x
}

func (c *RandomString) Uint64() uint64 {
	c.mu.Lock()
	x := c.r.Uint64()
	c.mu.Unlock()
	return x
}
