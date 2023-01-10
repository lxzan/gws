package internal

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestNewBufferWithCap(t *testing.T) {
	var as = assert.New(t)
	for i := 0; i < 10; i++ {
		var n = Numeric.Intn(256)
		var buf = NewBufferWithCap(uint8(n))
		as.Equal(n, buf.Cap())
	}
}

func TestBuffer_Reset(t *testing.T) {
	var as = assert.New(t)
	var buf = NewBuffer(nil)
	for i := 0; i < 10; i++ {
		buf.Reset()
		var n = AlphabetNumeric.Intn(1024)
		_, _ = buf.Write(AlphabetNumeric.Generate(n))
		as.Equal(buf.Cap()-n, buf.Available())
	}
}

func TestBytesBuffer_Write(t *testing.T) {
	var as = assert.New(t)

	t.Run("batch write", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			var s1 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(1024))
			var s2 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(512))
			var s3 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(256))
			var buf = NewBuffer(nil)
			buf.Write(s1)
			buf.Write(s2)
			buf.Write(s3)
			var s4 = string(s1) + string(s2) + string(s3)
			as.Equal(len(s4), buf.Len())
			as.Equal(s4, string(buf.Bytes()))
		}
	})

	t.Run("nil", func(t *testing.T) {
		var buf = NewBuffer(nil)
		_, _ = buf.Write([]byte("hello"))
		as.Equal("hello", string(buf.Bytes()))
	})

	t.Run("overflow", func(t *testing.T) {
		var b = make([]byte, 0, 5)
		b = append(b, 'h', 'e', 'l', 'l', 'o')
		var buf = NewBuffer(b)
		_, _ = buf.Write([]byte("world"))
		as.Equal("helloworld", string(buf.Bytes()))
	})
}

func TestBytesBuffer_Read(t *testing.T) {
	var as = assert.New(t)
	t.Run("batch read", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			var buf = NewBuffer(nil)
			var s1 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(8 * 1024))
			buf.Write(s1)
			s2, err := io.ReadAll(buf)
			if err != nil {
				t.Fail()
				return
			}
			if string(s1) != string(s2) {
				t.Fail()
				return
			}
		}
	})

	t.Run("read part", func(t *testing.T) {
		var buf = bytes.NewBufferString("hello")
		var s = make([]byte, 2)
		_, _ = buf.Read(s)
		if string(s) != "he" || string(buf.Bytes()) != "llo" {
			t.Fail()
		}
	})

	t.Run("read nil", func(t *testing.T) {
		var buf = NewBuffer(nil)
		var p = make([]byte, 2)
		_, err := buf.Read(p)
		as.Error(err)
	})

	t.Run("read overflow", func(t *testing.T) {
		var buf = NewBuffer([]byte("oh"))
		var p0 = make([]byte, 4)
		n, err := buf.Read(p0)
		as.NoError(err)
		as.Equal(2, n)

		var p1 = make([]byte, 4)
		_, err = buf.Read(p1)
		as.Error(err)
	})
}
