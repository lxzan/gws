package gws

import (
	"bytes"
	"compress/flate"
	"testing"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func TestFlate(t *testing.T) {
	var as = assert.New(t)

	t.Run("ok", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			var cps = newCompressor(flate.BestSpeed)
			var dps = newDecompressor()
			var n = internal.AlphabetNumeric.Intn(1024)
			var rawText = internal.AlphabetNumeric.Generate(n)
			var compressedBuf = bytes.NewBufferString("")
			if err := cps.Compress(rawText, compressedBuf); err != nil {
				as.NoError(err)
				return
			}

			var buf = bytes.NewBufferString("")
			buf.Write(compressedBuf.Bytes())
			plainText, _, err := dps.Decompress(buf)
			if err != nil {
				as.NoError(err)
				return
			}
			as.Equal(string(rawText), plainText.String())
		}
	})

	t.Run("deflate error", func(t *testing.T) {
		var cps = newCompressor(flate.BestSpeed)
		var dps = newDecompressor()
		var n = internal.AlphabetNumeric.Intn(1024)
		var rawText = internal.AlphabetNumeric.Generate(n)
		var compressedBuf = bytes.NewBufferString("")
		if err := cps.Compress(rawText, compressedBuf); err != nil {
			as.NoError(err)
			return
		}

		var buf = bytes.NewBufferString("")
		buf.Write(compressedBuf.Bytes())
		buf.WriteString("1234")
		_, _, err := dps.Decompress(buf)
		as.Error(err)
	})
}
