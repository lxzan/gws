package internal

import (
	"bytes"
	"sync"
)

// BufferPool 结构体定义了一个缓冲区池
// BufferPool struct defines a buffer pool
type BufferPool struct {
	// begin 表示缓冲区池的起始大小
	// begin indicates the starting size of the buffer pool
	begin int

	// end 表示缓冲区池的结束大小
	// end indicates the ending size of the buffer pool
	end int

	// shards 是一个映射，键是缓冲区大小，值是对应大小的 sync.Pool
	// shards is a map where the key is the buffer size and the value is a sync.Pool for that size
	shards map[int]*sync.Pool
}

// NewBufferPool 创建一个内存池
// NewBufferPool creates a memory pool
// left 和 right 表示内存池的区间范围，它们将被转换为 2 的 n 次幂
// left and right indicate the interval range of the memory pool, they will be transformed into pow(2, n)
// 小于 left 的情况下，Get 方法将返回至少 left 字节的缓冲区；大于 right 的情况下，Put 方法不会回收缓冲区
// Below left, the Get method will return at least left bytes; above right, the Put method will not reclaim the buffer
func NewBufferPool(left, right uint32) *BufferPool {
	// 计算 begin 和 end，分别为 left 和 right 向上取整到 2 的 n 次幂的值
	// Calculate begin and end, which are the ceiling values of left and right to the nearest power of 2
	var begin, end = int(binaryCeil(left)), int(binaryCeil(right))

	// 初始化 BufferPool 结构体
	// Initialize the BufferPool struct
	var p = &BufferPool{
		begin:  begin,
		end:    end,
		shards: make(map[int]*sync.Pool),
	}

	// 遍历从 begin 到 end 的所有 2 的 n 次幂的值
	// Iterate over all powers of 2 from begin to end
	for i := begin; i <= end; i *= 2 {
		// 将当前容量赋值给局部变量 capacity
		// Assign the current capacity to the local variable capacity
		capacity := i

		// 为当前容量创建一个 sync.Pool，并将其添加到 shards 映射中
		// Create a sync.Pool for the current capacity and add it to the shards map
		p.shards[i] = &sync.Pool{
			// 定义当池中没有可用缓冲区时创建新缓冲区的函数
			// Define the function to create a new buffer when there are no available buffers in the pool
			New: func() any { return bytes.NewBuffer(make([]byte, 0, capacity)) },
		}
	}

	// 返回初始化后的 BufferPool
	// Return the initialized BufferPool
	return p
}

// Put 将缓冲区返回到内存池
// Put returns the buffer to the memory pool
func (p *BufferPool) Put(b *bytes.Buffer) {
	// 如果缓冲区不为空
	// If the buffer is not nil
	if b != nil {
		// 检查缓冲区的容量是否在 shards 映射中
		// Check if the buffer's capacity is in the shards map
		if pool, ok := p.shards[b.Cap()]; ok {
			// 将缓冲区放回对应容量的池中
			// Put the buffer back into the pool of the corresponding capacity
			pool.Put(b)
		}
	}
}

// Get 从内存池中获取一个至少 n 字节的缓冲区
// Get fetches a buffer from the memory pool, of at least n bytes
func (p *BufferPool) Get(n int) *bytes.Buffer {
	// 计算所需的缓冲区大小，取 n 和 begin 中较大的值，并向上取整到 2 的 n 次幂
	// Calculate the required buffer size, taking the larger of n and begin, and rounding up to the nearest power of 2
	var size = Max(int(binaryCeil(uint32(n))), p.begin)

	// 检查所需大小的缓冲区池是否存在于 shards 映射中
	// Check if the buffer pool of the required size exists in the shards map
	if pool, ok := p.shards[size]; ok {
		// 从池中获取一个缓冲区
		// Get a buffer from the pool
		b := pool.Get().(*bytes.Buffer)

		// 如果缓冲区的容量小于所需大小，则扩展缓冲区
		// If the buffer's capacity is less than the required size, grow the buffer
		if b.Cap() < size {
			b.Grow(size)
		}

		// 重置缓冲区
		// Reset the buffer
		b.Reset()

		// 返回缓冲区
		// Return the buffer
		return b
	}

	// 如果所需大小的缓冲区池不存在，则创建一个新的缓冲区
	// If the buffer pool of the required size does not exist, create a new buffer
	return bytes.NewBuffer(make([]byte, 0, n))
}

// binaryCeil 将给定的 uint32 值向上取整到最近的 2 的幂
// binaryCeil rounds up the given uint32 value to the nearest power of 2
func binaryCeil(v uint32) uint32 {
	// 首先将 v 减 1，以处理 v 本身已经是 2 的幂的情况
	// First, decrement v by 1 to handle the case where v is already a power of 2
	v--

	// 将 v 的每一位与其右边的位进行或运算，逐步填充所有低位
	// Perform bitwise OR operations to fill all lower bits
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16

	// 最后将 v 加 1，得到大于或等于原始 v 的最小 2 的幂
	// Finally, increment v by 1 to get the smallest power of 2 greater than or equal to the original v
	v++

	// 返回结果
	// Return the result
	return v
}

// NewPool 创建一个新的泛型池
// NewPool creates a new generic pool
func NewPool[T any](f func() T) *Pool[T] {
	// 返回一个包含 sync.Pool 的 Pool 结构体
	// Return a Pool struct containing a sync.Pool
	return &Pool[T]{p: sync.Pool{New: func() any { return f() }}}
}

// Pool 是一个泛型池结构体
// Pool is a generic pool struct
type Pool[T any] struct {
	p sync.Pool // 内嵌的 sync.Pool
}

// Put 将一个值放入池中
// Put puts a value into the pool
func (c *Pool[T]) Put(v T) {
	c.p.Put(v) // 调用 sync.Pool 的 Put 方法
}

// Get 从池中获取一个值
// Get gets a value from the pool
func (c *Pool[T]) Get() T {
	return c.p.Get().(T) // 调用 sync.Pool 的 Get 方法并进行类型断言
}
