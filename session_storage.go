package gws

import (
	"sync"
)

// SessionStorage 会话存储接口
type SessionStorage interface {
	// Len 返回键值对数量
	Len() int

	// Load 根据键获取值, 存在则返回 (value, true), 否则返回 (nil, false)
	Load(key string) (value any, exist bool)

	// Delete 删除指定键的键值对
	Delete(key string)

	// Store 存储键值对
	Store(key string, value any)

	// Range 遍历, 函数返回 false 时提前终止
	Range(f func(key string, value any) bool)
}

// newSmap 创建 smap 实例
func newSmap() *smap {
	return &smap{data: make(map[string]any)}
}

// smap 基于 map 的会话存储实现
type smap struct {
	sync.Mutex
	data map[string]any
}

// Len 返回键值对数量
func (c *smap) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.data)
}

// Load 根据键获取值
func (c *smap) Load(key string) (value any, exist bool) {
	c.Lock()
	defer c.Unlock()
	value, exist = c.data[key]
	return
}

// Delete 删除指定键
func (c *smap) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, key)
}

// Store 存储键值对
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
type ConcurrentMap[K comparable, V any] struct {
	m sync.Map
}

// NewConcurrentMap 创建并发安全映射
func NewConcurrentMap[K comparable, V any]() *ConcurrentMap[K, V] {
	return &ConcurrentMap[K, V]{}
}

// Len 返回元素数量
func (c *ConcurrentMap[K, V]) Len() int {
	var length int
	c.m.Range(func(_, _ any) bool {
		length++
		return true
	})
	return length
}

// Load 返回键对应的值, ok 表示是否找到
func (c *ConcurrentMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := c.m.Load(key)
	if !ok {
		return value, false
	}
	return v.(V), true
}

// Delete 删除指定键
func (c *ConcurrentMap[K, V]) Delete(key K) {
	c.m.Delete(key)
}

// Store 设置键值对
func (c *ConcurrentMap[K, V]) Store(key K, value V) {
	c.m.Store(key, value)
}

// Range 遍历, f 返回 false 时停止
func (c *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	c.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
