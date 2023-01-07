package gws

import "sync"

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
