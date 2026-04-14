package gws

import (
	"sync"
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

// ConcurrentMap 并发安全的映射结构
// concurrency-safe map structure
type ConcurrentMap[K comparable, V any] struct {
	m sync.Map
}

// NewConcurrentMap 创建一个新的并发安全映射
// creates a new concurrency-safe map
func NewConcurrentMap[K comparable, V any]() *ConcurrentMap[K, V] {
	return &ConcurrentMap[K, V]{}
}

// Len 返回映射中的元素数量
// Len returns the number of elements in the map
func (c *ConcurrentMap[K, V]) Len() int {
	var length int
	c.m.Range(func(_, _ any) bool {
		length++
		return true
	})
	return length
}

// Load 返回映射中键对应的值，如果不存在则返回 nil
// returns the value stored in the map for a key, or nil if no value is present
// ok 结果表示是否在映射中找到了值
// The ok result indicates whether the value was found in the map
func (c *ConcurrentMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := c.m.Load(key)
	if !ok {
		return value, false
	}
	return v.(V), true
}

// Delete 删除键对应的值
// Delete deletes the value for a key
func (c *ConcurrentMap[K, V]) Delete(key K) {
	c.m.Delete(key)
}

// Store 设置键对应的值
// sets the value for a key
func (c *ConcurrentMap[K, V]) Store(key K, value V) {
	c.m.Store(key, value)
}

// Range 遍历
// 如果 f 返回 false，遍历停止
// If f returns false, range stops the iteration
func (c *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	c.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
