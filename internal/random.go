package internal

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// SplitMix64 对输入值执行 splitmix64 混合变换, 返回分布均匀的64位值.
// 变换是双射: 不同输入必然产生不同输出, 0输入对应0输出.
func SplitMix64(x uint64) uint64 {
	x ^= x >> 30 //nolint:mnd
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27 //nolint:mnd
	x *= 0x94D049BB133111EB
	x ^= x >> 31 //nolint:mnd
	return x
}

var seedCounter uint64

// NextSeed 返回非零且互不相同的种子值, 用于初始化相互独立的随机源
func NextSeed() uint64 {
	return SplitMix64(atomic.AddUint64(&seedCounter, 1))
}

// RandomString 随机字符串生成器
type RandomString struct {
	mu     sync.Mutex
	r      *rand.Rand
	layout string
}

var (
	// AlphabetNumeric 包含字母和数字字符集的 RandomString 实例
	AlphabetNumeric = &RandomString{
		layout: "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mu:     sync.Mutex{},
	}

	// Numeric 仅包含数字字符集的 RandomString 实例
	Numeric = &RandomString{
		layout: "0123456789",
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mu:     sync.Mutex{},
	}
)

// Generate 生成一个长度为 n 的随机字节切片
func (c *RandomString) Generate(n int) []byte {
	c.mu.Lock()
	var b = make([]byte, n)
	var length = len(c.layout)
	for i := range n {
		var idx = c.r.Intn(length)
		b[i] = c.layout[idx]
	}
	c.mu.Unlock()
	return b
}

// Intn 返回一个 [0, n) 范围内的随机整数
func (c *RandomString) Intn(n int) int {
	c.mu.Lock()
	x := c.r.Intn(n)
	c.mu.Unlock()
	return x
}

// Uint32 返回一个随机的 uint32 值
func (c *RandomString) Uint32() uint32 {
	c.mu.Lock()
	x := c.r.Uint32()
	c.mu.Unlock()
	return x
}

// Uint64 返回一个随机的 uint64 值
func (c *RandomString) Uint64() uint64 {
	c.mu.Lock()
	x := c.r.Uint64()
	c.mu.Unlock()
	return x
}
