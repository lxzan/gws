package internal

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIOUtil(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var reader = strings.NewReader("hello")
		var p = make([]byte, 5)
		var err = ReadN(reader, p)
		as.Nil(err)
	})

	t.Run("", func(t *testing.T) {
		var writer = bytes.NewBufferString("")
		var err = WriteN(writer, nil)
		as.NoError(err)
	})

	t.Run("", func(t *testing.T) {
		var writer = bytes.NewBufferString("")
		var p = []byte("hello")
		var err = WriteN(writer, p)
		as.NoError(err)
	})
}

func TestBuffers_WriteTo(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var b = Buffers([][]byte{
			[]byte("he"),
			[]byte("llo"),
		})
		var w = bytes.NewBufferString("")
		b.WriteTo(w)
		n, _ := b.WriteTo(w)
		assert.Equal(t, w.String(), "hellohello")
		assert.Equal(t, n, int64(5))
		assert.Equal(t, b.Len(), 5)
		assert.True(t, b.CheckEncoding(true, 1))
	})

	t.Run("", func(t *testing.T) {
		var conn, _ = net.Pipe()
		_ = conn.Close()
		var b = Buffers([][]byte{
			[]byte("he"),
			[]byte("llo"),
		})
		_, err := b.WriteTo(conn)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		var str = "你好"
		var b = Buffers([][]byte{
			[]byte("he"),
			[]byte(str[2:]),
		})
		assert.True(t, b.CheckEncoding(true, 1))
	})
}

func TestBytes_WriteTo(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var b = Bytes([]byte("hello"))
		var w = bytes.NewBufferString("")
		b.WriteTo(w)
		n, _ := b.WriteTo(w)
		assert.Equal(t, w.String(), "hellohello")
		assert.Equal(t, n, int64(5))
		assert.Equal(t, b.Len(), 5)
	})

	t.Run("", func(t *testing.T) {
		var str = "你好"
		var b = Bytes([]byte(str[2:]))
		assert.False(t, b.CheckEncoding(true, 1))
		assert.True(t, b.CheckEncoding(false, 1))
		assert.True(t, b.CheckEncoding(true, 2))
	})

	t.Run("WriteToBuffer", func(t *testing.T) {
		b1 := Bytes([]byte("hello"))
		buf1 := bytes.NewBuffer(nil)
		b1.WriteToBuffer(buf1)
		assert.Equal(t, "hello", buf1.String())

		b2 := Buffers([][]byte{[]byte("hello"), []byte(" world")})
		buf2 := bytes.NewBuffer(nil)
		b2.WriteToBuffer(buf2)
		assert.Equal(t, "hello world", buf2.String())
	})
}
