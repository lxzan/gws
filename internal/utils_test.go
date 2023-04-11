package internal

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"hash/fnv"
	"io"
	"strings"
	"testing"
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
	assert.Equal(t, h.Sum64(), FNV64(string(s)))
}

func TestIOUtil(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var dst = bytes.NewBuffer(nil)
		var src = bytes.NewBuffer(make([]byte, 0))
		var err = CopyN(dst, src, 0)
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		var dst = bytes.NewBuffer(nil)
		var src = bytes.NewBuffer(make([]byte, 6))
		var err = CopyN(dst, src, 12)
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		var reader = strings.NewReader("hello")
		var p = make([]byte, 0)
		var err = ReadN(reader, p, 0)
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		var reader = strings.NewReader("hello")
		var p = make([]byte, 5)
		var err = ReadN(reader, p, 10)
		as.Error(err)
	})

	t.Run("", func(t *testing.T) {
		var writer = bytes.NewBufferString("")
		var err = WriteN(writer, nil, 0)
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		var writer = bytes.NewBufferString("")
		var p = []byte("hello")
		var err = WriteN(writer, p, 5)
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		var buf1 = NewBufferWithCap(0)
		as.Equal(0, buf1.Cap())
		var buf2 = NewBufferWithCap(12)
		as.Equal(12, buf2.Cap())
	})
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
