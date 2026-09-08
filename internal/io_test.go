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
		var b = Buffers{
			[]byte("he"),
			[]byte("llo"),
		}
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
		var b = Buffers{
			[]byte("he"),
			[]byte("llo"),
		}
		_, err := b.WriteTo(conn)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		var str = "你好"
		var b = Buffers{
			[]byte("he"),
			[]byte(str[2:]),
		}
		assert.False(t, b.CheckEncoding(true, 1))
	})
}

func TestBuffers_CheckEncoding(t *testing.T) {
	var cn = []byte("中文")   // E4 B8 AD E6 96 87
	var emoji = []byte("😀") // F0 9F 98 80

	t.Run("valid sequence split across slices", func(t *testing.T) {
		assert.True(t, Buffers{cn[:1], cn[1:]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{cn[:2], cn[2:]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{cn[:1], cn[1:4], cn[4:]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{[]byte("hello "), cn[:1], cn[1:], []byte("!")}.CheckEncoding(true, 1))
	})

	t.Run("4-byte emoji split at every position", func(t *testing.T) {
		assert.True(t, Buffers{emoji[:1], emoji[1:]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{emoji[:2], emoji[2:]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{emoji[:3], emoji[3:]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{emoji[:1], emoji[1:2], emoji[2:3], emoji[3:]}.CheckEncoding(true, 1))
	})

	t.Run("empty buffers and slices", func(t *testing.T) {
		assert.True(t, Buffers{}.CheckEncoding(true, 1))
		assert.True(t, Buffers{nil, {}, cn[:1], {}, cn[1:], nil}.CheckEncoding(true, 1))
		assert.True(t, Buffers{nil}.CheckEncoding(true, 1))
	})

	t.Run("disabled or non-text opcode skips check", func(t *testing.T) {
		assert.True(t, Buffers{cn[:1], cn[1:]}.CheckEncoding(false, 1))
		assert.True(t, Buffers{{0x80}}.CheckEncoding(true, 2))
		assert.True(t, Buffers{cn[:1], cn[1:]}.CheckEncoding(true, 8))
	})

	t.Run("invalid sequence", func(t *testing.T) {
		assert.False(t, Buffers{[]byte("a"), {0x80}}.CheckEncoding(true, 1))
		assert.False(t, Buffers{cn[:2]}.CheckEncoding(true, 1))
		assert.False(t, Buffers{cn, emoji[:2]}.CheckEncoding(true, 1))
		assert.False(t, Buffers{cn[:1], {0x00}, cn[1:]}.CheckEncoding(true, 1))
		assert.False(t, Buffers{[]byte{0xFF}, cn[:1], cn[1:]}.CheckEncoding(true, 1))
		assert.False(t, Buffers{[]byte{0x80, 0x80, 0x80, 0x80}}.CheckEncoding(true, 1))
		assert.False(t, Buffers{{0xC0, 0x80}}.CheckEncoding(true, 1))
		assert.False(t, Buffers{{0xED, 0xA0, 0x80}}.CheckEncoding(true, 1))
	})

	t.Run("single slice behavior unchanged", func(t *testing.T) {
		assert.True(t, Buffers{cn}.CheckEncoding(true, 1))
		assert.False(t, Buffers{cn[:2]}.CheckEncoding(true, 1))
		assert.True(t, Buffers{[]byte("hello")}.CheckEncoding(true, 1))
	})
}

func TestBuffers_CheckEncoding_ZeroAlloc(t *testing.T) {
	var cn = []byte("中文")
	var emoji = []byte("😀")
	var ok bool

	t.Run("fast path", func(t *testing.T) {
		var b = Buffers{[]byte("hello"), cn, emoji}
		var allocs = testing.AllocsPerRun(100, func() {
			ok = b.CheckEncoding(true, 1)
		})
		assert.True(t, ok)
		assert.Zero(t, allocs)
	})

	t.Run("split path", func(t *testing.T) {
		var b = Buffers{cn[:1], cn[1:4], cn[4:], emoji[:2], emoji[2:]}
		var allocs = testing.AllocsPerRun(100, func() {
			ok = b.CheckEncoding(true, 1)
		})
		assert.True(t, ok)
		assert.Zero(t, allocs)
	})
}

func TestBytes_WriteTo(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var b = Bytes("hello")
		var w = bytes.NewBufferString("")
		b.WriteTo(w)
		n, _ := b.WriteTo(w)
		assert.Equal(t, w.String(), "hellohello")
		assert.Equal(t, n, int64(5))
		assert.Equal(t, b.Len(), 5)
	})

	t.Run("", func(t *testing.T) {
		var str = "你好"
		var b = Bytes(str[2:])
		assert.False(t, b.CheckEncoding(true, 1))
		assert.True(t, b.CheckEncoding(false, 1))
		assert.True(t, b.CheckEncoding(true, 2))
	})
}
