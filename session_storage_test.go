package gws

import (
	"testing"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	var as = assert.New(t)
	var m1 = make(map[string]interface{})
	var m2 = &sliceMap{}
	var count = internal.AlphabetNumeric.Intn(1000)
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	var keys = make([]string, 0)
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

func TestSliceMap(t *testing.T) {
	var as = assert.New(t)
	var m = new(sliceMap)
	m.Store("hong", 1)
	m.Store("mei", 2)
	m.Store("ming", 3)
	{
		v, _ := m.Load("hong")
		as.Equal(1, v)
	}
	{
		m.Delete("hong")
		v, ok := m.Load("hong")
		as.Equal(false, ok)
		as.Nil(v)

		m.Store("hong", 4)
		v, _ = m.Load("hong")
		as.Equal(4, v)
	}
}

func TestMap_Range(t *testing.T) {
	var as = assert.New(t)
	var m1 = make(map[interface{}]interface{})
	var m2 = &sliceMap{}
	var count = 1000
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	{
		var keys []interface{}
		m2.Range(func(key string, value interface{}) bool {
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
		m2.Range(func(key string, value interface{}) bool {
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
	var m1 = make(map[string]interface{})
	var m2 = NewConcurrentMap[string, uint32](5)
	var count = internal.AlphabetNumeric.Intn(1000)
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	var keys = make([]string, 0)
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
	var m2 = NewConcurrentMap[string, uint32](13)
	var count = 1000
	for i := 0; i < count; i++ {
		var key = string(internal.AlphabetNumeric.Generate(10))
		var val = internal.AlphabetNumeric.Uint32()
		m1[key] = val
		m2.Store(key, val)
	}

	{
		var keys []interface{}
		m2.Range(func(key string, value uint32) bool {
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
		m2.Range(func(key string, value uint32) bool {
			v, ok := m1[key]
			as.Equal(true, ok)
			as.Equal(v, value)
			keys = append(keys, key)
			return true
		})
		as.Equal(1000, len(keys))
	}
}

func TestHash(t *testing.T) {
	m := NewConcurrentMap[string, uint32](8)
	m.hash("1")

	m.hash(int(1))
	m.hash(int64(1))
	m.hash(int32(1))
	m.hash(int16(1))
	m.hash(int8(1))

	m.hash(uint(1))
	m.hash(uint64(1))
	m.hash(uint32(1))
	m.hash(uint16(1))
	m.hash(uint8(1))

	assert.Equal(t, uint64(0), m.hash(map[string]string{}))

	m = NewConcurrentMap[string, uint32](0)
	assert.Equal(t, uint64(16), m.segments)
}
