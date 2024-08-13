package gws

import (
	"sync"

	"github.com/dolthub/maphash"
	"github.com/lxzan/gws/internal"
)

// SessionStorage 接口定义了会话存储的基本操作
// The SessionStorage interface defines basic operations for session storage
type SessionStorage interface {
	// Len 返回存储中的键值对数量
	// Len returns the number of key-value pairs in the storage
	Len() int

	// Load 根据键获取值，如果键存在则返回值和 true，否则返回 nil 和 false
	// Load retrieves the value for a given key. If the key exists, it returns the value and true; otherwise, it returns nil and false
	Load(key string) (value any, exist bool)

	// Delete 根据键删除存储中的键值对
	// Delete removes the key-value pair from the storage for a given key
	Delete(key string)

	// Store 存储键值对
	// Store saves the key-value pair in the storage
	Store(key string, value any)

	// Range 遍历存储中的所有键值对，并对每个键值对执行给定的函数
	// Range iterates over all key-value pairs in the storage and executes the given function for each pair
	Range(f func(key string, value any) bool)
}

// newSmap 创建并返回一个新的 smap 实例
// newSmap creates and returns a new smap instance
func newSmap() *smap {
	return &smap{data: make(map[string]any)}
}

// smap 是一个基于 map 的会话存储实现
// smap is a map-based implementation of the session storage
type smap struct {
	sync.Mutex
	data map[string]any
}

// Len 返回存储中的键值对数量
// Len returns the number of key-value pairs in the storage
func (c *smap) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.data)
}

// Load 根据键获取值，如果键存在则返回值和 true，否则返回 nil 和 false
// Load retrieves the value for a given key. If the key exists, it returns the value and true; otherwise, it returns nil and false
func (c *smap) Load(key string) (value any, exist bool) {
	c.Lock()
	defer c.Unlock()
	value, exist = c.data[key]
	return
}

// Delete 根据键删除存储中的键值对
// Delete removes the key-value pair from the storage for a given key
func (c *smap) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, key)
}

// Store 存储键值对
// Store saves the key-value pair in the storage
func (c *smap) Store(key string, value any) {
	c.Lock()
	defer c.Unlock()
	c.data[key] = value
}

// Range 遍历存储中的所有键值对，并对每个键值对执行给定的函数
// Range iterates over all key-value pairs in the storage and executes the given function for each pair
func (c *smap) Range(f func(key string, value any) bool) {
	c.Lock()
	defer c.Unlock()

	for k, v := range c.data {
		if !f(k, v) {
			return
		}
	}
}

type (
	// ConcurrentMap 是一个并发安全的映射结构
	// ConcurrentMap is a concurrency-safe map structure
	ConcurrentMap[K comparable, V any] struct {
		// hasher 用于计算键的哈希值
		// hasher is used to compute the hash value of keys
		hasher maphash.Hasher[K]

		// num 表示分片的数量
		// num represents the number of shardings
		num uint64

		// shardings 存储实际的分片映射
		// shardings stores the actual sharding maps
		shardings []*Map[K, V]
	}
)

// NewConcurrentMap 创建一个新的并发安全映射
// NewConcurrentMap creates a new concurrency-safe map
// arg0 表示分片的数量；arg1 表示分片的初始化容量
// arg0 represents the number of shardings; arg1 represents the initialized capacity of a sharding.
func NewConcurrentMap[K comparable, V any](size ...uint64) *ConcurrentMap[K, V] {
	// 确保 size 至少有两个元素，默认值为 0
	// Ensure size has at least two elements, defaulting to 0
	size = append(size, 0, 0)

	// 获取分片数量和初始化容量
	// Get the number of shardings and the initial capacity
	num, capacity := size[0], size[1]

	// 将分片数量调整为二进制数
	// Adjust the number of shardings to a binary number
	num = internal.ToBinaryNumber(internal.SelectValue(num <= 0, 16, num))

	// 创建 ConcurrentMap 实例
	// Create a ConcurrentMap instance
	var cm = &ConcurrentMap[K, V]{
		hasher:    maphash.NewHasher[K](),
		num:       num,
		shardings: make([]*Map[K, V], num),
	}

	// 初始化每个分片
	// Initialize each sharding
	for i := range cm.shardings {
		cm.shardings[i] = NewMap[K, V](int(capacity))
	}

	// 返回创建的 ConcurrentMap 实例
	// Return the created ConcurrentMap instance
	return cm
}

// GetSharding 返回一个键的分片映射
// GetSharding returns a map sharding for a key
// 分片中的操作是无锁的，需要手动加锁
// The operations inside the sharding are lockless and need to be locked manually.
func (c *ConcurrentMap[K, V]) GetSharding(key K) *Map[K, V] {
	// 计算键的哈希值
	// Calculate the hash code for the key
	var hashCode = c.hasher.Hash(key)

	// 计算分片索引
	// Calculate the sharding index
	var index = hashCode & (c.num - 1)

	// 返回对应索引的分片
	// Return the sharding at the calculated index
	return c.shardings[index]
}

// Len 返回映射中的元素数量
// Len returns the number of elements in the map
func (c *ConcurrentMap[K, V]) Len() int {
	var length = 0

	// 遍历所有分片并累加它们的长度
	// Iterate over all shardings and sum their lengths
	for _, b := range c.shardings {
		b.Lock()
		length += b.Len()
		b.Unlock()
	}

	// 返回总长度
	// Return the total length
	return length
}

// Load 返回映射中键对应的值，如果不存在则返回 nil
// Load returns the value stored in the map for a key, or nil if no value is present
// ok 结果表示是否在映射中找到了值
// The ok result indicates whether the value was found in the map
func (c *ConcurrentMap[K, V]) Load(key K) (value V, ok bool) {
	// 获取键对应的分片
	// Get the sharding for the key
	var b = c.GetSharding(key)

	// 加锁并加载值
	// Lock and load the value
	b.Lock()
	value, ok = b.Load(key)
	b.Unlock()

	// 返回值和状态
	// Return the value and status
	return
}

// Delete 删除键对应的值
// Delete deletes the value for a key
func (c *ConcurrentMap[K, V]) Delete(key K) {
	// 获取键对应的分片
	// Get the sharding for the key
	var b = c.GetSharding(key)

	// 加锁并删除值
	// Lock and delete the value
	b.Lock()
	b.Delete(key)
	b.Unlock()
}

// Store 设置键对应的值
// Store sets the value for a key
func (c *ConcurrentMap[K, V]) Store(key K, value V) {
	// 获取键对应的分片
	// Get the sharding for the key
	var b = c.GetSharding(key)

	// 加锁并存储值
	// Lock and store the value
	b.Lock()
	b.Store(key, value)
	b.Unlock()
}

// Range 依次为映射中的每个键和值调用 f
// Range calls f sequentially for each key and value present in the map
// 如果 f 返回 false，遍历停止
// If f returns false, range stops the iteration
func (c *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	var next = true

	// 包装回调函数以检查是否继续
	// Wrap the callback function to check whether to continue
	var cb = func(k K, v V) bool {
		next = f(k, v)
		return next
	}

	// 遍历所有分片并调用回调函数
	// Iterate over all shardings and call the callback function
	for i := uint64(0); i < c.num && next; i++ {
		var b = c.shardings[i]
		b.Lock()
		b.Range(cb)
		b.Unlock()
	}
}

// Map 是一个线程安全的泛型映射类型。
// Map is a thread-safe generic map type.
type Map[K comparable, V any] struct {
	// Mutex 用于确保并发访问的安全性
	// Mutex is used to ensure safety for concurrent access
	sync.Mutex

	// m 是实际存储键值对的底层映射
	// m is the underlying map that stores key-value pairs
	m map[K]V
}

// NewMap 创建一个新的 Map 实例。
// NewMap creates a new instance of Map.
// size 参数用于指定初始容量，如果未提供则默认为 0。
// The size parameter is used to specify the initial capacity, defaulting to 0 if not provided.
func NewMap[K comparable, V any](size ...int) *Map[K, V] {
	// 初始化容量为 0
	// Initialize capacity to 0
	var capacity = 0

	// 如果提供了 size 参数，则使用第一个值作为容量
	// If the size parameter is provided, use the first value as the capacity
	if len(size) > 0 {
		capacity = size[0]
	}

	// 创建一个新的 Map 实例
	// Create a new instance of Map
	c := new(Map[K, V])

	// 初始化底层映射，使用指定的容量
	// Initialize the underlying map with the specified capacity
	c.m = make(map[K]V, capacity)

	// 返回创建的 Map 实例
	// Return the created Map instance
	return c
}

// Len 返回 Map 中的元素数量。
// Len returns the number of elements in the Map.
func (c *Map[K, V]) Len() int {
	return len(c.m)
}

// Load 从 Map 中加载指定键的值。
// Load loads the value for the specified key from the Map.
func (c *Map[K, V]) Load(key K) (value V, ok bool) {
	value, ok = c.m[key]
	return
}

// Delete 从 Map 中删除指定键的值。
// Delete deletes the value for the specified key from the Map.
func (c *Map[K, V]) Delete(key K) {
	delete(c.m, key)
}

// Store 将指定键值对存储到 Map 中。
// Store stores the specified key-value pair in the Map.
func (c *Map[K, V]) Store(key K, value V) {
	c.m[key] = value
}

// Range 遍历 Map 中的所有键值对，并对每个键值对执行指定的函数。
// 如果函数返回 false，遍历将提前终止。
// Range iterates over all key-value pairs in the Map and executes the specified function for each pair.
// If the function returns false, the iteration stops early.
func (c *Map[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.m {
		if !f(k, v) {
			return
		}
	}
}
