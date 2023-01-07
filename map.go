package gws

import "sync"

func NewMap() *Map {
	return &Map{mu: sync.RWMutex{}, d: make(map[string]interface{})}
}

type Map struct {
	mu sync.RWMutex
	d  map[string]interface{}
}

func (c *Map) Len() int {
	c.mu.RLock()
	n := len(c.d)
	c.mu.RUnlock()
	return n
}

func (c *Map) Load(key string) (value interface{}, exist bool) {
	c.mu.RLock()
	value, exist = c.d[key]
	c.mu.RUnlock()
	return
}

// Delete deletes the value for a key.
func (c *Map) Delete(key string) {
	c.mu.Lock()
	delete(c.d, key)
	c.mu.Unlock()
}

// Store sets the value for a key.
func (c *Map) Store(key string, value interface{}) {
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
