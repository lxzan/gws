package gws

import (
	"sync"

	"github.com/lxzan/gws/internal"
)

// SessionStorage because sync.Map is not easy to debug, so I implemented my own map.
// if you don't like it, use sync.Map instead.
type SessionStorage interface {
	Load(key string) (value interface{}, exist bool)
	Delete(key string)
	Store(key string, value interface{})
	Range(f func(key string, value interface{}) bool)
}

type (
	sliceMap struct {
		sync.RWMutex
		data []kv
	}

	kv struct {
		deleted bool
		key     string
		value   interface{}
	}
)

func (c *sliceMap) Len() int {
	c.RLock()
	defer c.RUnlock()
	var n = len(c.data)
	for _, v := range c.data {
		if v.deleted {
			n--
		}
	}
	return n
}

func (c *sliceMap) Load(key string) (value interface{}, exist bool) {
	c.RLock()
	defer c.RUnlock()
	for _, v := range c.data {
		if v.key == key && !v.deleted {
			return v.value, true
		}
	}
	return nil, false
}

func (c *sliceMap) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	for i, v := range c.data {
		if v.key == key {
			c.data[i].value = nil
			c.data[i].deleted = true
			return
		}
	}
}

func (c *sliceMap) Store(key string, value interface{}) {
	c.Lock()
	defer c.Unlock()

	for i, v := range c.data {
		if v.key == key {
			c.data[i].value = value
			c.data[i].deleted = false
			return
		}
	}

	c.data = append(c.data, kv{
		deleted: false,
		key:     key,
		value:   value,
	})
}

func (c *sliceMap) Range(f func(key string, value interface{}) bool) {
	c.Lock()
	defer c.Unlock()

	for _, v := range c.data {
		if !v.deleted {
			if !f(v.key, v.value) {
				return
			}
		}
	}
}

/*
ConcurrentMap
used to store websocket connections in the IM server
用来存储IM等服务的连接
*/
type (
	ConcurrentMap[K comparable, V any] struct {
		segments uint64
		buckets  []*bucket[K, V]
	}

	bucket[K comparable, V any] struct {
		sync.RWMutex
		m map[K]V
	}
)

func NewConcurrentMap[K comparable, V any](segments uint64) *ConcurrentMap[K, V] {
	if segments == 0 {
		segments = 16
	} else {
		var num = uint64(1)
		for num < segments {
			num *= 2
		}
		segments = num
	}
	var cm = &ConcurrentMap[K, V]{segments: segments, buckets: make([]*bucket[K, V], segments, segments)}
	for i, _ := range cm.buckets {
		cm.buckets[i] = &bucket[K, V]{m: make(map[K]V)}
	}
	return cm
}

func (c *ConcurrentMap[K, V]) hash(key interface{}) uint64 {
	switch k := key.(type) {
	case string:
		return internal.FNV64(k)
	case int:
		return uint64(k)
	case int64:
		return uint64(k)
	case int32:
		return uint64(k)
	case int16:
		return uint64(k)
	case int8:
		return uint64(k)
	case uint:
		return uint64(k)
	case uint64:
		return k
	case uint32:
		return uint64(k)
	case uint16:
		return uint64(k)
	case uint8:
		return uint64(k)
	default:
		return 0
	}
}

func (c *ConcurrentMap[K, V]) getBucket(key K) *bucket[K, V] {
	var hashCode = c.hash(key)
	var index = hashCode & (c.segments - 1)
	return c.buckets[index]
}

func (c *ConcurrentMap[K, V]) Len() int {
	var length = 0
	for _, b := range c.buckets {
		b.RLock()
		length += len(b.m)
		b.RUnlock()
	}
	return length
}

func (c *ConcurrentMap[K, V]) Load(key K) (value V, exist bool) {
	var b = c.getBucket(key)
	b.RLock()
	value, exist = b.m[key]
	b.RUnlock()
	return
}

func (c *ConcurrentMap[K, V]) Delete(key K) {
	var b = c.getBucket(key)
	b.Lock()
	delete(b.m, key)
	b.Unlock()
}

func (c *ConcurrentMap[K, V]) Store(key K, value V) {
	var b = c.getBucket(key)
	b.Lock()
	b.m[key] = value
	b.Unlock()
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (c *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	for _, b := range c.buckets {
		b.RLock()
		for k, v := range b.m {
			if !f(k, v) {
				b.RUnlock()
				return
			}
		}
		b.RUnlock()
	}
}
