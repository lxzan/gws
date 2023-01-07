package gws

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"sync"
	"testing"
	"unsafe"
)

func BenchmarkNewBuffer(b *testing.B) {
	var str = internal.AlphabetNumeric.Generate(1024)
	var bs = *(*[]byte)(unsafe.Pointer(&str))
	for i := 0; i < b.N; i++ {
		bytes.NewBuffer(bs)
	}
}

func BenchmarkCompress(b *testing.B) {
	var s = internal.AlphabetNumeric.Generate(1024)
	var buf = bytes.NewBuffer(nil)
	fw, _ := flate.NewWriter(nil, -2)
	for i := 0; i < b.N; i++ {
		buf.Reset()
		fw.Reset(buf)
		fw.Write(s)
		fw.Flush()
		fw.Close()
	}
}

func BenchmarkDeCompress(b *testing.B) {
	var s = internal.AlphabetNumeric.Generate(1024)
	var buf = bytes.NewBuffer(nil)
	fw, _ := flate.NewWriter(buf, -2)
	fw.Write(s)
	fw.Flush()
	fw.Close()
	fr := flate.NewReader(nil)

	var result = bytes.NewBuffer(nil)
	result.Grow(1024)
	for i := 0; i < b.N; i++ {
		r := bytes.NewBuffer(buf.Bytes())
		fr.(flate.Resetter).Reset(r, nil)
		result.Reset()
		io.Copy(result, fr)
	}
}

func BenchmarkMask(b *testing.B) {
	var s1 = internal.AlphabetNumeric.Generate(1280)
	var s2 = s1
	var key [4]byte
	binary.LittleEndian.PutUint32(key[:4], internal.AlphabetNumeric.Uint32())
	for i := 0; i < b.N; i++ {
		maskXOR(s2, key[:4])
	}
}

func BenchmarkSyncMap_Store(b *testing.B) {
	var m = sync.Map{}
	for i := 0; i < b.N; i++ {
		var key = string(internal.AlphabetNumeric.Generate(16))
		m.Store(key, 1)
	}
}

func BenchmarkSyncMap_Load(b *testing.B) {
	var m = sync.Map{}
	for i := 0; i < 10000; i++ {
		var key = string(internal.AlphabetNumeric.Generate(4))
		m.Store(key, 1)
	}
	for i := 0; i < b.N; i++ {
		var key = string(internal.AlphabetNumeric.Generate(4))
		m.Load(key)
	}
}

func TestMask(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var s1 = internal.AlphabetNumeric.Generate(1280)
		var s2 = make([]byte, len(s1))
		copy(s2, s1)

		var key = make([]byte, 4, 4)
		binary.LittleEndian.PutUint32(key, internal.AlphabetNumeric.Uint32())
		maskXOR(s1, key)
		internal.MaskByByte(s2, key)
		for i, _ := range s1 {
			if s1[i] != s2[i] {
				t.Fail()
			}
		}
	}
}
