package gws

import (
	"sync"

	"github.com/dolthub/maphash"
	"github.com/lxzan/gws/internal"
)

type SessionStorage interface {
	Len() int
	Load(key string) (value any, exist bool)
	Delete(key string)
	Store(key string, value any)
	Range(f func(key string, value any) bool)
}

func newSmap() *smap { return &smap{data: make(map[string]any)} }

type smap struct {
	sync.Mutex
	data map[string]any
}

func (c *smap) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.data)
}

func (c *smap) Load(key string) (value any, exist bool) {
	c.Lock()
	defer c.Unlock()
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
	ConcurrentMap[K comparable, V any] struct {
		hasher   maphash.Hasher[K]
		sharding uint64
		buckets  []*Map[K, V]
	}
)

// NewConcurrentMap create a new concurrency-safe map
// arg0 represents the number of shardings; arg1 represents the initialized capacity of a sharding.
func NewConcurrentMap[K comparable, V any](size ...uint64) *ConcurrentMap[K, V] {
	sharding, capacity := uint64(0), uint64(0)
	if len(size) >= 1 {
		sharding = size[0]
	}
	if len(size) >= 2 {
		capacity = size[1]
	}
	sharding = internal.SelectValue(sharding <= 0, 16, sharding)
	sharding = internal.ToBinaryNumber(sharding)
	var cm = &ConcurrentMap[K, V]{
		hasher:   maphash.NewHasher[K](),
		sharding: sharding,
		buckets:  make([]*Map[K, V], sharding),
	}
	for i, _ := range cm.buckets {
		cm.buckets[i] = &Map[K, V]{m: make(map[K]V, capacity)}
	}
	return cm
}

// GetSharding returns a map sharding for a key
func (c *ConcurrentMap[K, V]) GetSharding(key K) *Map[K, V] {
	var hashCode = c.hasher.Hash(key)
	var index = hashCode & (c.sharding - 1)
	return c.buckets[index]
}

// Len returns the number of elements in the map
func (c *ConcurrentMap[K, V]) Len() int {
	var length = 0
	for _, b := range c.buckets {
		b.Lock()
		length += b.Len()
		b.Unlock()
	}
	return length
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (c *ConcurrentMap[K, V]) Load(key K) (value V, ok bool) {
	var b = c.GetSharding(key)
	b.Lock()
	value, ok = b.Load(key)
	b.Unlock()
	return
}

// Delete deletes the value for a key.
func (c *ConcurrentMap[K, V]) Delete(key K) {
	var b = c.GetSharding(key)
	b.Lock()
	b.Delete(key)
	b.Unlock()
}

// Store sets the value for a key.
func (c *ConcurrentMap[K, V]) Store(key K, value V) {
	var b = c.GetSharding(key)
	b.Lock()
	b.Store(key, value)
	b.Unlock()
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (c *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	var next = true
	var cb = func(k K, v V) bool {
		next = f(k, v)
		return next
	}
	for i := uint64(0); i < c.sharding && next; i++ {
		var b = c.buckets[i]
		b.Lock()
		b.Range(cb)
		b.Unlock()
	}
}

type Map[K comparable, V any] struct {
	sync.Mutex
	m map[K]V
}

func (c *Map[K, V]) Len() int { return len(c.m) }

func (c *Map[K, V]) Load(key K) (value V, ok bool) {
	value, ok = c.m[key]
	return
}

func (c *Map[K, V]) Delete(key K) { delete(c.m, key) }

func (c *Map[K, V]) Store(key K, value V) { c.m[key] = value }

func (c *Map[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.m {
		if !f(k, v) {
			return
		}
	}
}
