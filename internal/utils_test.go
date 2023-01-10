package internal

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestNewMaskKey(t *testing.T) {
	var key = NewMaskKey()
	assert.Equal(t, 4, len(key))
}

func TestComputeAcceptKey(t *testing.T) {
	var s = ComputeAcceptKey("PUurdSuLQj/6n4NFf/rA7A==")
	assert.Equal(t, "HmIbwxkcLxq+A+3qnlBVtT7Bjgg=", s)
}

func TestCloneHeader(t *testing.T) {
	var as = assert.New(t)
	var h1 = http.Header{}
	h1.Set("X-S1", string(AlphabetNumeric.Generate(32)))
	h1.Set("X-S2", string(AlphabetNumeric.Generate(64)))
	var h2 = CloneHeader(h1)
	b1, _ := json.Marshal(h1)
	b2, _ := json.Marshal(h2)
	as.Equal(len(b1), len(b2))
	as.Equal(h1.Get("X-S1"), h2.Get("X-S1"))
	var h3 = h1
	var p1 = fmt.Sprintf("%p", h1)
	var p2 = fmt.Sprintf("%p", h2)
	var p3 = fmt.Sprintf("%p", h3)
	as.Equal(p1, p3)
	as.NotEqual(p1, p2)
}

func TestMethodExists(t *testing.T) {
	var as = assert.New(t)

	t.Run("exist", func(t *testing.T) {
		var b = NewBuffer(nil)
		_, ok := MethodExists(b, "Write")
		as.Equal(true, ok)
	})

	t.Run("not exist", func(t *testing.T) {
		var b = NewBuffer(nil)
		_, ok := MethodExists(b, "XXX")
		as.Equal(false, ok)
	})

	t.Run("non struct", func(t *testing.T) {
		var m = make(map[string]interface{})
		_, ok := MethodExists(m, "Delete")
		as.Equal(false, ok)
	})

	t.Run("nil", func(t *testing.T) {
		var v interface{}
		_, ok := MethodExists(v, "XXX")
		as.Equal(false, ok)
	})
}
