package internal

import (
	"math/rand"
	"sync"
	"time"
)

// RandomString 结构体用于生成随机字符串
// RandomString struct is used to generate random strings
type RandomString struct {
	// mu 是一个互斥锁，用于保护并发访问
	// mu is a mutex to protect concurrent access
	mu sync.Mutex

	// r 是一个随机数生成器
	// r is a random number generator
	r *rand.Rand

	// layout 是用于生成随机字符串的字符集
	// layout is the character set used to generate random strings
	layout string
}

var (
	// AlphabetNumeric 是一个包含字母和数字字符集的 RandomString 实例
	// AlphabetNumeric is a RandomString instance with an alphanumeric character set
	AlphabetNumeric = &RandomString{
		// layout 包含数字和大小写字母
		// layout contains numbers and uppercase and lowercase letters
		layout: "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",

		// r 使用当前时间的纳秒数作为种子创建一个新的随机数生成器
		// r creates a new random number generator seeded with the current time in nanoseconds
		r: rand.New(rand.NewSource(time.Now().UnixNano())),

		// mu 初始化为一个新的互斥锁
		// mu is initialized as a new mutex
		mu: sync.Mutex{},
	}

	// Numeric 是一个仅包含数字字符集的 RandomString 实例
	// Numeric is a RandomString instance with a numeric character set
	Numeric = &RandomString{
		// layout 仅包含数字
		// layout contains only numbers
		layout: "0123456789",

		// r 使用当前时间的纳秒数作为种子创建一个新的随机数生成器
		// r creates a new random number generator seeded with the current time in nanoseconds
		r: rand.New(rand.NewSource(time.Now().UnixNano())),

		// mu 初始化为一个新的互斥锁
		// mu is initialized as a new mutex
		mu: sync.Mutex{},
	}
)

// Generate 生成一个长度为 n 的随机字节切片
// Generate generates a random byte slice of length n
func (c *RandomString) Generate(n int) []byte {
	// 加锁以确保线程安全
	// Lock to ensure thread safety
	c.mu.Lock()

	// 创建一个长度为 n 的字节切片
	// Create a byte slice of length n
	var b = make([]byte, n, n)

	// 获取字符集的长度
	// Get the length of the character set
	var length = len(c.layout)

	// 生成随机字节
	// Generate random bytes
	for i := 0; i < n; i++ {
		// 从字符集中随机选择一个字符的索引
		// Randomly select an index from the character set
		var idx = c.r.Intn(length)

		// 将字符集中的字符赋值给字节切片
		// Assign the character from the character set to the byte slice
		b[i] = c.layout[idx]
	}

	// 解锁
	// Unlock
	c.mu.Unlock()

	// 返回生成的字节切片
	// Return the generated byte slice
	return b
}

// Intn 返回一个 [0, n) 范围内的随机整数
// Intn returns a random integer in the range [0, n)
func (c *RandomString) Intn(n int) int {
	// 加锁以确保线程安全
	// Lock to ensure thread safety
	c.mu.Lock()

	// 生成随机整数
	// Generate a random integer
	x := c.r.Intn(n)

	// 解锁
	// Unlock
	c.mu.Unlock()

	// 返回生成的随机整数
	// Return the generated random integer
	return x
}

// Uint32 返回一个随机的 uint32 值
// Uint32 returns a random uint32 value
func (c *RandomString) Uint32() uint32 {
	// 加锁以确保线程安全
	// Lock to ensure thread safety
	c.mu.Lock()

	// 生成随机的 uint32 值
	// Generate a random uint32 value
	x := c.r.Uint32()

	// 解锁
	// Unlock
	c.mu.Unlock()

	// 返回生成的随机 uint32 值
	// Return the generated random uint32 value
	return x
}

// Uint64 返回一个随机的 uint64 值
// Uint64 returns a random uint64 value
func (c *RandomString) Uint64() uint64 {
	// 加锁以确保线程安全
	// Lock to ensure thread safety
	c.mu.Lock()

	// 生成随机的 uint64 值
	// Generate a random uint64 value
	x := c.r.Uint64()

	// 解锁
	// Unlock
	c.mu.Unlock()

	// 返回生成的随机 uint64 值
	// Return the generated random uint64 value
	return x
}
