package gws

import (
	"github.com/lxzan/gws/internal"
	"sync"
)

// SessionStorage because sync.Map is not easy to debug, so I implemented my own map.
// if you don't like it, you can also use sync.Map instead.
type SessionStorage interface {
	Load(key interface{}) (value interface{}, exist bool)
	Delete(key interface{})
	Store(key interface{}, value interface{})
	Range(f func(key, value interface{}) bool)
}

func NewMap() *Map {
	return &Map{mu: sync.RWMutex{}, d: make(map[interface{}]interface{})}
}

type Map struct {
	mu sync.RWMutex
	d  map[interface{}]interface{}
}

func (c *Map) Len() int {
	c.mu.RLock()
	n := len(c.d)
	c.mu.RUnlock()
	return n
}

func (c *Map) Load(key interface{}) (value interface{}, exist bool) {
	c.mu.RLock()
	value, exist = c.d[key]
	c.mu.RUnlock()
	return
}

// Delete deletes the value for a key.
func (c *Map) Delete(key interface{}) {
	c.mu.Lock()
	delete(c.d, key)
	c.mu.Unlock()
}

// Store sets the value for a key.
func (c *Map) Store(key interface{}, value interface{}) {
	c.mu.Lock()
	c.d[key] = value
	c.mu.Unlock()
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (c *Map) Range(f func(key, value interface{}) bool) {
	c.mu.RLock()
	for k, v := range c.d {
		if ok := f(k, v); !ok {
			break
		}
	}
	c.mu.RUnlock()
}

/*
ConcurrentMap
used to store websocket connections in the IM server
用来存储IM等服务的连接
*/
type (
	ConcurrentMap struct {
		segments uint64
		buckets  []*bucket
	}

	bucket struct {
		sync.RWMutex
		m map[interface{}]interface{}
	}
)

func NewConcurrentMap(segments uint64) *ConcurrentMap {
	if segments == 0 {
		segments = 16
	} else {
		var num = uint64(1)
		for num < segments {
			num *= 2
		}
		segments = num
	}
	var cm = &ConcurrentMap{segments: segments, buckets: make([]*bucket, segments, segments)}
	for i, _ := range cm.buckets {
		cm.buckets[i] = &bucket{m: make(map[interface{}]interface{})}
	}
	return cm
}

func (c *ConcurrentMap) hash(key interface{}) uint64 {
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

func (c *ConcurrentMap) getBucket(key interface{}) *bucket {
	var hashCode = c.hash(key)
	var index = hashCode & (c.segments - 1)
	return c.buckets[index]
}

func (c *ConcurrentMap) Len() int {
	var length = 0
	for _, b := range c.buckets {
		b.RLock()
		length += len(b.m)
		b.RUnlock()
	}
	return length
}

func (c *ConcurrentMap) Load(key interface{}) (value interface{}, exist bool) {
	var b = c.getBucket(key)
	b.RLock()
	value, exist = b.m[key]
	b.RUnlock()
	return
}

func (c *ConcurrentMap) Delete(key interface{}) {
	var b = c.getBucket(key)
	b.Lock()
	delete(b.m, key)
	b.Unlock()
}

func (c *ConcurrentMap) Store(key interface{}, value interface{}) {
	var b = c.getBucket(key)
	b.Lock()
	b.m[key] = value
	b.Unlock()
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (c *ConcurrentMap) Range(f func(key interface{}, value interface{}) bool) {
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
