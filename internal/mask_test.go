package internal

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskByByte(t *testing.T) {
	var data = []byte("hello")
	MaskByByte(data, []byte{0xa, 0xb, 0xc, 0xd})
	assert.Equal(t, "626e606165", hex.EncodeToString(data))
}

func TestMask(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var n = AlphabetNumeric.Intn(1024)
		var s1 = AlphabetNumeric.Generate(n)
		var s2 = make([]byte, len(s1))
		copy(s2, s1)

		var key = make([]byte, 4, 4)
		binary.LittleEndian.PutUint32(key, AlphabetNumeric.Uint32())
		MaskXOR(s1, key)
		MaskByByte(s2, key)
		for i, _ := range s1 {
			if s1[i] != s2[i] {
				t.Fail()
			}
		}
	}
}
