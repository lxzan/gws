package gws

import (
	"github.com/lxzan/gws/internal"
	"sync"
)

type SessionStorage interface {
	Load(key string) (value any, exist bool)
	Delete(key string)
	Store(key string, value any)
	Range(f func(key string, value any) bool)
}

func newSmap() *smap { return &smap{data: make(map[string]any)} }

type smap struct {
	sync.RWMutex
	data map[string]any
}

func (c *smap) Len() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.data)
}

func (c *smap) Load(key string) (value any, exist bool) {
	c.RLock()
	defer c.RUnlock()
	value, exist = c.data[key]
	return
}

func (c *smap) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, key)
}

func (c *smap) Store(key string, value any) {
	c.Lock()
	defer c.Unlock()
	c.data[key] = value
}

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
	Comparable interface {
		~string | ~int | ~int64 | ~int32 | ~uint | ~uint64 | ~uint32
	}

	ConcurrentMap[K Comparable, V any] struct {
		segments uint64
		buckets  []*bucket[K, V]
	}

	bucket[K Comparable, V any] struct {
		sync.Mutex
		m map[K]V
	}
)

func NewConcurrentMap[K Comparable, V any](segments uint64) *ConcurrentMap[K, V] {
	segments = internal.SelectValue(segments == 0, 16, segments)
	segments = internal.ToBinaryNumber(segments)
	var cm = &ConcurrentMap[K, V]{segments: segments, buckets: make([]*bucket[K, V], segments, segments)}
	for i, _ := range cm.buckets {
		cm.buckets[i] = &bucket[K, V]{m: make(map[K]V)}
	}
	return cm
}

func (c *ConcurrentMap[K, V]) hash(key any) uint64 {
	switch k := key.(type) {
	case string:
		return internal.FnvString(k)
	case int:
		return internal.FnvNumber(k)
	case int64:
		return internal.FnvNumber(k)
	case int32:
		return internal.FnvNumber(k)
	case uint:
		return internal.FnvNumber(k)
	case uint64:
		return internal.FnvNumber(k)
	case uint32:
		return internal.FnvNumber(k)
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
		b.Lock()
		length += len(b.m)
		b.Unlock()
	}
	return length
}

func (c *ConcurrentMap[K, V]) Load(key K) (value V, exist bool) {
	var b = c.getBucket(key)
	b.Lock()
	value, exist = b.m[key]
	b.Unlock()
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
		b.Lock()
		for k, v := range b.m {
			if !f(k, v) {
				b.Unlock()
				return
			}
		}
		b.Unlock()
	}
}
