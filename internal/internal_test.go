package internal

import (
	"bytes"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestError(t *testing.T) {
	var as = assert.New(t)
	t.Run("", func(t *testing.T) {
		var code = StatusCode(1000)
		as.Equal(uint16(1000), code.Uint16())
		as.Equal(hex.EncodeToString(code.Bytes()), "03e8")
		as.Equal(code.Error() != "", true)
	})

	t.Run("", func(t *testing.T) {
		var code = StatusCode(0)
		as.Equal(hex.EncodeToString(code.Bytes()), "0000")
	})

	t.Run("", func(t *testing.T) {
		var err error = NewError(CloseGoingAway, io.EOF)
		as.Equal(err.Error() != "", true)
	})
}

func TestBuffer_ReadFrom(t *testing.T) {
	var b = Buffer{Buffer: bytes.NewBuffer(nil)}
	b.ReadFrom()
}

func TestRandomString_Uint32(t *testing.T) {
	Numeric.Uint32()
}
