package internal

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"io"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestStringToBytes(t *testing.T) {
	var s1 = string(AlphabetNumeric.Generate(32))
	var s2 = string(StringToBytes(s1))
	assert.Equal(t, s1, s2)
}

func TestComputeAcceptKey(t *testing.T) {
	var s = ComputeAcceptKey("PUurdSuLQj/6n4NFf/rA7A==")
	assert.Equal(t, "HmIbwxkcLxq+A+3qnlBVtT7Bjgg=", s)
}

func TestMethodExists(t *testing.T) {
	var as = assert.New(t)

	t.Run("exist", func(t *testing.T) {
		var b = bytes.NewBuffer(nil)
		_, ok := MethodExists(b, "Write")
		as.Equal(true, ok)
	})

	t.Run("not exist", func(t *testing.T) {
		var b = bytes.NewBuffer(nil)
		_, ok := MethodExists(b, "XXX")
		as.Equal(false, ok)
	})

	t.Run("non struct", func(t *testing.T) {
		var m = make(map[string]any)
		_, ok := MethodExists(m, "Delete")
		as.Equal(false, ok)
	})

	t.Run("nil", func(t *testing.T) {
		var v any
		_, ok := MethodExists(v, "XXX")
		as.Equal(false, ok)
	})
}

func BenchmarkStringToBytes(b *testing.B) {
	var s = string(AlphabetNumeric.Generate(1024))
	var buffer = bytes.NewBuffer(make([]byte, 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = io.Copy(buffer, bytes.NewBuffer(StringToBytes(s)))
	}
}

func BenchmarkStringReader(b *testing.B) {
	var s = string(AlphabetNumeric.Generate(1024))
	var buffer = bytes.NewBuffer(make([]byte, 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = io.Copy(buffer, strings.NewReader(s))
	}
}

func TestFNV64(t *testing.T) {
	var s = AlphabetNumeric.Generate(16)
	var h = fnv.New64()
	_, _ = h.Write(s)
	assert.Equal(t, h.Sum64(), FnvString(string(s)))
	_ = FnvNumber(1234)
}

func TestNewMaskKey(t *testing.T) {
	var key = NewMaskKey()
	assert.Equal(t, 4, len(key))
}

func TestMaskByByte(t *testing.T) {
	var data = []byte("hello")
	MaskByByte(data, []byte{0xa, 0xb, 0xc, 0xd})
	assert.Equal(t, "626e606165", hex.EncodeToString(data))
}

func TestMask(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var n = AlphabetNumeric.Intn(1024)
		var s1 = AlphabetNumeric.Generate(n)
		var s2 = make([]byte, len(s1))
		copy(s2, s1)

		var key = make([]byte, 4, 4)
		binary.LittleEndian.PutUint32(key, AlphabetNumeric.Uint32())
		MaskXOR(s1, key)
		MaskByByte(s2, key)
		for i, _ := range s1 {
			if s1[i] != s2[i] {
				t.Fail()
			}
		}
	}
}

func TestSplit(t *testing.T) {
	var sep = "/"
	assert.ElementsMatch(t, []string{"api", "v1"}, Split("/api/v1", sep))
	assert.ElementsMatch(t, []string{"api", "v1"}, Split("/api/v1/", sep))
	assert.ElementsMatch(t, []string{"ming", "hong", "hu"}, Split("ming/ hong/ hu", sep))
	assert.ElementsMatch(t, []string{"ming", "hong", "hu"}, Split("/ming/ hong/ hu/ ", sep))
	assert.ElementsMatch(t, []string{"ming", "hong", "hu"}, Split("\nming/ hong/ hu\n", sep))
	assert.ElementsMatch(t, []string{"ming", "hong", "hu"}, Split("\nming, hong, hu\n", ","))
}

func TestInCollection(t *testing.T) {
	var as = assert.New(t)
	as.Equal(true, InCollection("hong", []string{"lang", "hong"}))
	as.Equal(true, InCollection("lang", []string{"lang", "hong"}))
	as.Equal(false, InCollection("long", []string{"lang", "hong"}))
}

func TestRandomString_Uint64(t *testing.T) {
	AlphabetNumeric.Uint64()
}

func TestHttpHeaderEqual(t *testing.T) {
	assert.Equal(t, true, HttpHeaderEqual("WebSocket", "websocket"))
	assert.Equal(t, false, HttpHeaderEqual("WebSocket@", "websocket"))
}

func TestHttpHeaderContains(t *testing.T) {
	assert.Equal(t, true, HttpHeaderContains("WebSocket", "websocket"))
	assert.Equal(t, true, HttpHeaderContains("WebSocket@", "websocket"))
}

func TestSelectInt(t *testing.T) {
	assert.Equal(t, 1, SelectValue(true, 1, 2))
	assert.Equal(t, 2, SelectValue(false, 1, 2))
}

func TestToBinaryNumber(t *testing.T) {
	assert.Equal(t, 8, ToBinaryNumber(7))
	assert.Equal(t, 1, ToBinaryNumber(0))
	assert.Equal(t, 128, ToBinaryNumber(120))
	assert.Equal(t, 1024, ToBinaryNumber(1024))
}

func TestGetIntersectionElem(t *testing.T) {
	{
		a := []string{"chat", "stock", "excel"}
		b := []string{"stock", "fx"}
		assert.Equal(t, "stock", GetIntersectionElem(a, b))
	}
	{
		a := []string{"chat", "stock", "excel"}
		b := []string{"fx"}
		assert.Equal(t, "", GetIntersectionElem(a, b))
	}
	{
		a := []string{}
		b := []string{"fx"}
		assert.Equal(t, "", GetIntersectionElem(a, b))
		assert.Equal(t, "", GetIntersectionElem(b, a))
	}
	{
		b := []string{"fx"}
		assert.Equal(t, "", GetIntersectionElem(nil, b))
		assert.Equal(t, "", GetIntersectionElem(b, nil))
	}
}

func TestResetBuffer(t *testing.T) {
	{
		var buffer = bytes.NewBufferString("hello")
		var name = reflect.TypeOf(buffer).Elem().Field(0).Name
		assert.Equal(t, "buf", name)
	}

	{
		var buf = bytes.NewBufferString("")
		BufferReset(buf, []byte("hello"))
		assert.Equal(t, "hello", buf.String())

		var p = buf.Bytes()
		var sh1 = (*reflect.SliceHeader)(unsafe.Pointer(&p))
		var sh2 = (*reflect.SliceHeader)(unsafe.Pointer(buf))
		assert.Equal(t, sh1.Data, sh2.Data)
	}
}

func TestWithDefault(t *testing.T) {
	assert.Equal(t, WithDefault(0, 1), 1)
	assert.Equal(t, WithDefault(2, 1), 2)
}

func TestBinaryPow(t *testing.T) {
	assert.Equal(t, BinaryPow(0), 1)
	assert.Equal(t, BinaryPow(1), 2)
	assert.Equal(t, BinaryPow(3), 8)
	assert.Equal(t, BinaryPow(10), 1024)
}

func TestMin(t *testing.T) {
	assert.Equal(t, Min(1, 2), 1)
	assert.Equal(t, Min(4, 3), 3)
}

func TestMax(t *testing.T) {
	assert.Equal(t, Max(1, 2), 2)
	assert.Equal(t, Max(4, 3), 4)
}

func TestIsSameSlice(t *testing.T) {
	assert.True(t, IsSameSlice(
		[]int{1, 2, 3},
		[]int{1, 2, 3},
	))

	assert.False(t, IsSameSlice(
		[]int{1, 2, 3},
		[]int{1, 2},
	))

	assert.False(t, IsSameSlice(
		[]int{1, 2, 3},
		[]int{1, 2, 4},
	))
}
