package websocket

import (
	"bytes"
	"compress/flate"
	"github.com/lxzan/websocket/internal"
	"io"
	"math/rand"
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
		fw.Write(*(*[]byte)(unsafe.Pointer(&s)))
		fw.Flush()
		fw.Close()
	}
}

func BenchmarkDeCompress(b *testing.B) {
	var s = internal.AlphabetNumeric.Generate(1024)
	var buf = bytes.NewBuffer(nil)
	fw, _ := flate.NewWriter(buf, -2)
	fw.Write(*(*[]byte)(unsafe.Pointer(&s)))
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
	var s1 = internal.AlphabetNumeric.Generate(128)
	var s2 = []byte(s1)
	for i := 0; i < b.N; i++ {
		withMask(s2, rand.Uint32())
	}
}
