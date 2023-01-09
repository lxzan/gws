package gws

import (
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMap(t *testing.T) {
	var as = assert.New(t)
	var m1 = make(map[interface{}]interface{})
	var m2 = NewMap()
	var count = internal.AlphabetNumeric.Intn(1000)
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	var keys = make([]interface{}, 0)
	for k, _ := range m1 {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys)/2; i++ {
		delete(m1, keys[i])
		m2.Delete(keys[i])
	}

	for k, v := range m1 {
		v1, ok := m2.Load(k)
		as.Equal(true, ok)
		as.Equal(v, v1)
	}
	as.Equal(len(m1), m2.Len())
}

func TestMap_Range(t *testing.T) {
	var as = assert.New(t)
	var m1 = make(map[interface{}]interface{})
	var m2 = NewMap()
	var count = 1000
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	{
		var keys []interface{}
		m2.Range(func(key interface{}, value interface{}) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return len(keys) < 100
		})
		as.Equal(100, len(keys))
	}

	{
		var keys []interface{}
		m2.Range(func(key interface{}, value interface{}) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return true
		})
		as.Equal(1000, len(keys))
	}
}

func TestConcurrentMap(t *testing.T) {
	var as = assert.New(t)
	var m1 = make(map[interface{}]interface{})
	var m2 = NewConcurrentMap(5)
	var count = internal.AlphabetNumeric.Intn(1000)
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	var keys = make([]interface{}, 0)
	for k, _ := range m1 {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys)/2; i++ {
		delete(m1, keys[i])
		m2.Delete(keys[i])
	}

	for k, v := range m1 {
		v1, ok := m2.Load(k)
		as.Equal(true, ok)
		as.Equal(v, v1)
	}
	as.Equal(len(m1), m2.Len())
}

func TestConcurrentMap_Range(t *testing.T) {
	var as = assert.New(t)
	var m1 = make(map[interface{}]interface{})
	var m2 = NewConcurrentMap(13)
	var count = 1000
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	{
		var keys []interface{}
		m2.Range(func(key interface{}, value interface{}) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return len(keys) < 100
		})
		as.Equal(100, len(keys))
	}

	{
		var keys []interface{}
		m2.Range(func(key interface{}, value interface{}) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return true
		})
		as.Equal(1000, len(keys))
	}
}
