package internal

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestBytesBuffer_Write(t *testing.T) {
	var as = assert.New(t)
	for i := 0; i < 1000; i++ {
		var s1 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(1024))
		var s2 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(512))
		var s3 = AlphabetNumeric.Generate(AlphabetNumeric.Intn(256))
		var buf = NewBuffer(nil)
		buf.Write(s1)
		buf.Write(s2)
		buf.Write(s3)
		as.Equal(string(s1)+string(s2)+string(s3), string(buf.Bytes()))
	}
}

func TestBytesBuffer_Read(t *testing.T) {
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

	{
		var buf = bytes.NewBufferString("hello")
		var s = make([]byte, 2)
		_, _ = buf.Read(s)
		if string(s) != "he" || string(buf.Bytes()) != "llo" {
			t.Fail()
		}
	}
}
