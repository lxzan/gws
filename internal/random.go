package internal

import (
	"math/rand"
	"sync"
	"time"
)

// RandomString 随机字符串生成器
// random string generator
type RandomString struct {
	mu     sync.Mutex
	r      *rand.Rand
	layout string
}

var (
	// AlphabetNumeric 包含字母和数字字符集的 RandomString 实例
	// It's a RandomString instance with an alphanumeric character set
	AlphabetNumeric = &RandomString{
		layout: "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mu:     sync.Mutex{},
	}

	// Numeric 仅包含数字字符集的 RandomString 实例
	// It's a RandomString instance with a numeric character set
	Numeric = &RandomString{
		layout: "0123456789",
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mu:     sync.Mutex{},
	}
)

// Generate 生成一个长度为 n 的随机字节切片
// generates a random byte slice of length n
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

// Intn 返回一个 [0, n) 范围内的随机整数
// returns a random integer in the range [0, n)
func (c *RandomString) Intn(n int) int {
	c.mu.Lock()
	x := c.r.Intn(n)
	c.mu.Unlock()
	return x
}

// Uint32 返回一个随机的 uint32 值
// returns a random uint32 value
func (c *RandomString) Uint32() uint32 {
	c.mu.Lock()
	x := c.r.Uint32()
	c.mu.Unlock()
	return x
}

// Uint64 返回一个随机的 uint64 值
// returns a random uint64 value
func (c *RandomString) Uint64() uint64 {
	c.mu.Lock()
	x := c.r.Uint64()
	c.mu.Unlock()
	return x
}
