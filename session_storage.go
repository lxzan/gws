package gws

import (
	"sync"

	"github.com/dolthub/maphash"
	"github.com/lxzan/gws/internal"
)

// SessionStorage 会话存储
type SessionStorage interface {
	// Len 返回存储中的键值对数量
	// Returns the number of key-value pairs in the storage
	Len() int

	// Load 根据键获取值，如果键存在则返回值和 true，否则返回 nil 和 false
	// retrieves the value for a given key. If the key exists, it returns the value and true; otherwise, it returns nil and false
	Load(key string) (value any, exist bool)

	// Delete 根据键删除存储中的键值对
	// removes the key-value pair from the storage for a given key
	Delete(key string)

	// Store 存储键值对
	// saves the key-value pair in the storage
	Store(key string, value any)

	// Range 遍历
	// 如果函数返回 false，遍历将提前终止.
	// If the function returns false, the iteration stops early.
	Range(f func(key string, value any) bool)
}

// newSmap 创建并返回一个新的 smap 实例
// creates and returns a new smap instance
func newSmap() *smap {
	return &smap{data: make(map[string]any)}
}

// smap 基于 map 的会话存储实现
// map-based implementation of the session storage
type smap struct {
	sync.Mutex
	data map[string]any
}

// Len 返回存储中的键值对数量
// returns the number of key-value pairs in the storage
func (c *smap) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.data)
}

// Load 根据键获取值，如果键存在则返回值和 true，否则返回 nil 和 false
// retrieves the value for a given key. If the key exists, it returns the value and true; otherwise, it returns nil and false
func (c *smap) Load(key string) (value any, exist bool) {
	c.Lock()
	defer c.Unlock()
	value, exist = c.data[key]
	return
}

// Delete 根据键删除存储中的键值对
// removes the key-value pair from the storage for a given key
func (c *smap) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, key)
}

// Store 存储键值对
// saves the key-value pair in the storage
func (c *smap) Store(key string, value any) {
	c.Lock()
	defer c.Unlock()
	c.data[key] = value
}

// Range 遍历
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
	// ConcurrentMap 并发安全的映射结构
	// concurrency-safe map structure
	ConcurrentMap[K comparable, V any] struct {
		// hasher 用于计算键的哈希值
		// compute the hash value of keys
		hasher maphash.Hasher[K]

		// num 表示分片的数量
		// represents the number of shardings
		num uint64

		// shardings 存储实际的分片映射
		// stores the actual sharding maps
		shardings []*Map[K, V]
	}
)

// NewConcurrentMap 创建一个新的并发安全映射
// creates a new concurrency-safe map
// arg0 表示分片的数量；arg1 表示分片的初始化容量
// arg0 represents the number of shardings; arg1 represents the initialized capacity of a sharding.
func NewConcurrentMap[K comparable, V any](size ...uint64) *ConcurrentMap[K, V] {
	size = append(size, 0, 0)
	num, capacity := size[0], size[1]
	num = internal.ToBinaryNumber(internal.SelectValue(num <= 0, 16, num))
	var cm = &ConcurrentMap[K, V]{
		hasher:    maphash.NewHasher[K](),
		num:       num,
		shardings: make([]*Map[K, V], num),
	}
	for i, _ := range cm.shardings {
		cm.shardings[i] = NewMap[K, V](int(capacity))
	}
	return cm
}

// GetSharding 返回一个键的分片映射
// returns a map sharding for a key
// 分片中的操作是无锁的，需要手动加锁
// The operations inside the sharding are lockless and need to be locked manually.
func (c *ConcurrentMap[K, V]) GetSharding(key K) *Map[K, V] {
	var hashCode = c.hasher.Hash(key)
	var index = hashCode & (c.num - 1)
	return c.shardings[index]
}

// Len 返回映射中的元素数量
// Len returns the number of elements in the map
func (c *ConcurrentMap[K, V]) Len() int {
	var length = 0
	for _, b := range c.shardings {
		b.Lock()
		length += b.Len()
		b.Unlock()
	}
	return length
}

// Load 返回映射中键对应的值，如果不存在则返回 nil
// returns the value stored in the map for a key, or nil if no value is present
// ok 结果表示是否在映射中找到了值
// The ok result indicates whether the value was found in the map
func (c *ConcurrentMap[K, V]) Load(key K) (value V, ok bool) {
	var b = c.GetSharding(key)
	b.Lock()
	value, ok = b.Load(key)
	b.Unlock()
	return
}

// Delete 删除键对应的值
// Delete deletes the value for a key
func (c *ConcurrentMap[K, V]) Delete(key K) {
	var b = c.GetSharding(key)
	b.Lock()
	b.Delete(key)
	b.Unlock()
}

// Store 设置键对应的值
// sets the value for a key
func (c *ConcurrentMap[K, V]) Store(key K, value V) {
	var b = c.GetSharding(key)
	b.Lock()
	b.Store(key, value)
	b.Unlock()
}

// Range 遍历
// 如果 f 返回 false，遍历停止
// If f returns false, range stops the iteration
func (c *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	var next = true
	var cb = func(k K, v V) bool {
		next = f(k, v)
		return next
	}
	for i := uint64(0); i < c.num && next; i++ {
		var b = c.shardings[i]
		b.Lock()
		b.Range(cb)
		b.Unlock()
	}
}

// Map 线程安全的泛型映射类型.
// thread-safe generic map type.
type Map[K comparable, V any] struct {
	sync.Mutex
	m map[K]V
}

// NewMap 创建一个新的 Map 实例
// creates a new instance of Map
// size 参数用于指定初始容量，如果未提供则默认为 0
// The size parameter is used to specify the initial capacity, defaulting to 0 if not provided.
func NewMap[K comparable, V any](size ...int) *Map[K, V] {
	var capacity = 0
	if len(size) > 0 {
		capacity = size[0]
	}
	c := new(Map[K, V])
	c.m = make(map[K]V, capacity)
	return c
}

// Len 返回 Map 中的元素数量.
// Len returns the number of elements in the Map.
func (c *Map[K, V]) Len() int {
	return len(c.m)
}

// Load 从 Map 中加载指定键的值.
// Load loads the value for the specified key from the Map.
func (c *Map[K, V]) Load(key K) (value V, ok bool) {
	value, ok = c.m[key]
	return
}

// Delete 从 Map 中删除指定键的值.
// deletes the value for the specified key from the Map.
func (c *Map[K, V]) Delete(key K) {
	delete(c.m, key)
}

// Store 将指定键值对存储到 Map 中.
// stores the specified key-value pair in the Map.
func (c *Map[K, V]) Store(key K, value V) {
	c.m[key] = value
}

// Range 遍历
// 如果函数返回 false，遍历将提前终止.
// If the function returns false, the iteration stops early.
func (c *Map[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.m {
		if !f(k, v) {
			return
		}
	}
}
