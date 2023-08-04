package internal

import (
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
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
		as.Equal(hex.EncodeToString(code.Bytes()), "")
	})

	t.Run("", func(t *testing.T) {
		var err error = NewError(CloseGoingAway, io.EOF)
		as.Equal(err.Error() != "", true)
	})

	t.Run("", func(t *testing.T) {
		err1 := Errors(func() error {
			return nil
		})
		as.NoError(err1)

		err2 := Errors(func() error {
			return nil
		}, func() error {
			return errors.New("test")
		}, func() error {
			panic("fatal error")
		})
		as.Error(err2)
	})
}

func TestRandomString_Uint32(t *testing.T) {
	Numeric.Uint32()
}
