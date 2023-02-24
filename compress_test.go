package gws

import (
	"bytes"
	"compress/flate"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFlate(t *testing.T) {
	var as = assert.New(t)

	for i := 0; i < 100; i++ {
		var cps = newCompressor(flate.BestSpeed)
		var dps = newDecompressor()
		var n = internal.AlphabetNumeric.Intn(1024)
		var rawText = internal.AlphabetNumeric.Generate(n)
		var buf = bytes.NewBufferString("")
		buf.Write(rawText)
		compressedText, err := cps.Compress(buf)
		if err != nil {
			as.NoError(err)
			return
		}

		buf.Reset()
		buf.Write(compressedText.Bytes())
		plainText, err := dps.Decompress(buf)
		if err != nil {
			as.NoError(err)
			return
		}
		as.Equal(string(rawText), plainText.String())
	}
}
