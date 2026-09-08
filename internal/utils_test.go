package internal

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"io"
	"net/url"
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

func BenchmarkStringToBytes(b *testing.B) {
	var s = string(AlphabetNumeric.Generate(1024))
	var buffer = bytes.NewBuffer(make([]byte, 1024))
	b.ResetTimer()
	for range b.N {
		_, _ = io.Copy(buffer, bytes.NewBuffer(StringToBytes(s)))
	}
}

func BenchmarkStringReader(b *testing.B) {
	var s = string(AlphabetNumeric.Generate(1024))
	var buffer = bytes.NewBuffer(make([]byte, 1024))
	b.ResetTimer()
	for range b.N {
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

func TestMask(t *testing.T) {
	for range 1000 {
		var n = AlphabetNumeric.Intn(1024)
		var s1 = AlphabetNumeric.Generate(n)
		var s2 = make([]byte, len(s1))
		copy(s2, s1)

		var key = make([]byte, 4)
		binary.LittleEndian.PutUint32(key, AlphabetNumeric.Uint32())
		MaskXOR(s1, key)
		naiveMaskXOROffset(s2, key, 0)
		for i := range s1 {
			if s1[i] != s2[i] {
				t.Fail()
			}
		}
	}
}

func TestMaskXOROffset(t *testing.T) {
	for range 1000 {
		var n = AlphabetNumeric.Intn(1024) + 1
		var offset = AlphabetNumeric.Intn(4)
		var src = AlphabetNumeric.Generate(n)
		var want = make([]byte, len(src))
		copy(want, src)
		naiveMaskXOROffset(want, []byte{0xa, 0xb, 0xc, 0xd}, offset)

		var got = make([]byte, len(src))
		copy(got, src)
		MaskXOROffset(got, []byte{0xa, 0xb, 0xc, 0xd}, offset)
		assert.Equal(t, want, got)
	}
}

func naiveMaskXOROffset(content []byte, key []byte, offset int) {
	for i := range content {
		content[i] ^= key[(offset+i)&3]
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
		var typ = reflect.TypeOf(buffer).Elem()
		assert.Equal(t, "buf", typ.Field(0).Name)
		assert.Equal(t, "off", typ.Field(1).Name)
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

	t.Run("drained", func(t *testing.T) {
		var buf = bytes.NewBufferString("01234567")
		var sink = make([]byte, 8)
		var n, _ = buf.Read(sink)
		assert.Equal(t, 8, n)
		BufferReset(buf, []byte("hello"))
		var got = make([]byte, 16)
		n, err := buf.Read(got)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(got[:n]))
	})

	t.Run("partially read", func(t *testing.T) {
		var buf = bytes.NewBufferString("01234567")
		var sink = make([]byte, 4)
		var _, _ = buf.Read(sink)
		BufferReset(buf, []byte("hello"))
		assert.Equal(t, "hello", buf.String())
	})

	t.Run("write after reset", func(t *testing.T) {
		var buf = bytes.NewBufferString("01234567")
		var sink = make([]byte, 8)
		var _, _ = buf.Read(sink)
		BufferReset(buf, []byte("hello"))
		var _, _ = buf.WriteString(" world")
		assert.Equal(t, "hello world", buf.String())
	})

	t.Run("zero alloc", func(t *testing.T) {
		var buf = bytes.NewBuffer(nil)
		var p = []byte("hello")
		assert.Equal(t, float64(0), testing.AllocsPerRun(100, func() {
			BufferReset(buf, p)
		}))
	})
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

func TestIsIPv6(t *testing.T) {
	assert.False(t, IsIPv6("192.168.1.1"))
	assert.False(t, IsIPv6("google.com"))
	assert.True(t, IsIPv6("2001:0:2851:b9f0:3866:a7c6:871b:706a"))
}

func TestGetAddrFromURL(t *testing.T) {
	t.Run("", func(t *testing.T) {
		u, _ := url.Parse("wss://[2001:0:2851:b9f0:3866:a7c6:871b:706a]/connect")
		addr := GetAddrFromURL(u, true)
		assert.Equal(t, addr, "[2001:0:2851:b9f0:3866:a7c6:871b:706a]:443")
	})

	t.Run("", func(t *testing.T) {
		u, _ := url.Parse("wss://:9443")
		addr := GetAddrFromURL(u, true)
		assert.Equal(t, addr, "127.0.0.1:9443")
	})

	t.Run("", func(t *testing.T) {
		u, _ := url.Parse("wss://")
		addr := GetAddrFromURL(u, true)
		assert.Equal(t, addr, "127.0.0.1:443")
	})

	t.Run("", func(t *testing.T) {
		u, _ := url.Parse("ws://google.com/connect")
		addr := GetAddrFromURL(u, false)
		assert.Equal(t, addr, "google.com:80")
	})
}
