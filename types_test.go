package gws

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"testing"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

// newFrameHeader127 构造 lengthCode=127(不带mask)的帧头字节序列, 64位payload长度为n
// Builds a frame header byte sequence with lengthCode=127 (unmasked), 64-bit payload length n
func newFrameHeader127(n uint64) []byte {
	var src = make([]byte, 10)
	src[0] = 0x82 // FIN=1, opcode=binary
	src[1] = 127
	binary.BigEndian.PutUint64(src[2:10], n)
	return src
}

func TestFrameHeader_Parse_Case127_ValidLength(t *testing.T) {
	var as = assert.New(t)

	var fh = frameHeader{}
	var n, err = fh.Parse(bytes.NewReader(newFrameHeader127(65536)))
	as.NoError(err)
	as.Equal(65536, n)
}

// case-127 分支的长度守卫必须是平台自适应的: 对任意超过平台 int 上限的长度
// 返回协议错误, 防止 32 位平台上 int(n) 截断为负数后绕过 ReadMaxPayloadSize 检查.
// The case-127 length guard MUST be platform-adaptive: any length exceeding the
// platform's int limit is a protocol error, preventing int(n) from truncating to
// a negative value on 32-bit platforms, which would bypass ReadMaxPayloadSize.
func TestFrameHeader_Parse_Case127_PlatformBoundary(t *testing.T) {
	var as = assert.New(t)
	var is32Bit = strconv.IntSize == 32

	var parse = func(length uint64) (int, error) {
		var fh = frameHeader{}
		return fh.Parse(bytes.NewReader(newFrameHeader127(length)))
	}

	t.Run("accepts math.MaxInt32 on all platforms", func(t *testing.T) {
		var n, err = parse(math.MaxInt32)
		as.NoError(err)
		as.Equal(math.MaxInt32, n)
	})

	t.Run("0xFFFFFFF0: platform-dependent", func(t *testing.T) {
		var length int64 = 0xFFFFFFF0
		var n, err = parse(uint64(length))
		if is32Bit {
			as.Equal(internal.CloseProtocolError, err)
			as.Equal(0, n)
		} else {
			as.NoError(err)
			as.Equal(int(length), n)
		}
	})

	t.Run("math.MaxInt64: platform-dependent", func(t *testing.T) {
		var length int64 = math.MaxInt64
		var n, err = parse(uint64(length))
		if is32Bit {
			as.Equal(internal.CloseProtocolError, err)
			as.Equal(0, n)
		} else {
			as.NoError(err)
			as.Equal(int(length), n)
		}
	})

	t.Run("rejects n > math.MaxInt64 on all platforms", func(t *testing.T) {
		var n, err = parse(uint64(math.MaxInt64) + 1)
		as.Equal(internal.CloseProtocolError, err)
		as.Equal(0, n)
	})
}
