package gws

import (
	"github.com/klauspost/compress/flate"
	"github.com/stretchr/testify/assert"
	"testing"
)

//func TestFlate(t *testing.T) {
//	var as = assert.New(t)
//
//	t.Run("ok", func(t *testing.T) {
//		for i := 0; i < 100; i++ {
//			var cps = newCompressor(flate.BestSpeed)
//			var dps = newDecompressor()
//			var n = internal.AlphabetNumeric.Intn(1024)
//			var rawText = internal.AlphabetNumeric.Generate(n)
//			var compressedBuf = bytes.NewBufferString("")
//			if err := cps.Compress(rawText, compressedBuf); err != nil {
//				as.NoError(err)
//				return
//			}
//
//			var buf = bytes.NewBufferString("")
//			buf.Write(compressedBuf.Bytes())
//			plainText, err := dps.Decompress(buf)
//			if err != nil {
//				as.NoError(err)
//				return
//			}
//			as.Equal(string(rawText), plainText.String())
//		}
//	})
//
//	t.Run("deflate error", func(t *testing.T) {
//		var cps = newCompressor(flate.BestSpeed)
//		var dps = newDecompressor()
//		var n = internal.AlphabetNumeric.Intn(1024)
//		var rawText = internal.AlphabetNumeric.Generate(n)
//		var compressedBuf = bytes.NewBufferString("")
//		if err := cps.Compress(rawText, compressedBuf); err != nil {
//			as.NoError(err)
//			return
//		}
//
//		var buf = bytes.NewBufferString("")
//		buf.Write(compressedBuf.Bytes())
//		buf.WriteString("1234")
//		_, err := dps.Decompress(buf)
//		as.Error(err)
//	})
//}
//
//func TestDecompressor_Init(t *testing.T) {
//	var d = &decompressor{
//		b:  bytes.NewBuffer(internal.AlphabetNumeric.Generate(512 * 1024)),
//		fr: flate.NewReader(nil),
//	}
//	d.reset(nil)
//	assert.Equal(t, d.b.Cap(), 0)
//}

func TestSlideWindow(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var sw = new(slideWindow).initialize(3)
		sw.Write([]byte("abc"))
		assert.Equal(t, string(sw.dict), "abc")

		sw.Write([]byte("def"))
		assert.Equal(t, string(sw.dict), "abcdef")

		sw.Write([]byte("ghi"))
		assert.Equal(t, string(sw.dict), "bcdefghi")
	})

	t.Run("", func(t *testing.T) {
		var sw = new(slideWindow).initialize(3)
		sw.Write([]byte("abc"))
		assert.Equal(t, string(sw.dict), "abc")

		sw.Write([]byte("defgh123456789"))
		assert.Equal(t, string(sw.dict), "23456789")
	})
}

func TestDeflaterC_Negotiation(t *testing.T) {
	d := new(deflaterC)
	d.initialize(flate.BestSpeed, "permessage-deflate; client_no_context_takeover; client_max_window_bits=9")
	println(1)
}
