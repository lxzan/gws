package gws

import (
	"encoding/binary"
	"io"
)

func maskXOR(content []byte, key []byte) {
	var maskKey = binary.LittleEndian.Uint32(key)
	var key64 = uint64(maskKey)<<32 + uint64(maskKey)

	var n = len(content)
	var end = n - n&7

	var i = 0
	for i = 0; i < end; i += 8 {
		v := binary.LittleEndian.Uint64(content[i : i+8])
		binary.LittleEndian.PutUint64(content[i:i+8], v^key64)
	}
	for ; i < n; i++ {
		idx := i & 3
		content[i] ^= key[idx]
	}
}

func (c *Conn) readN(data []byte, n int) error {
	num, err := io.ReadFull(c.rbuf, data)
	if err != nil {
		return err
	}
	if num != n {
		return CloseGoingAway
	}
	return nil
}

func writeN(writer io.Writer, content []byte, n int) error {
	num, err := writer.Write(content)
	if err != nil {
		return err
	}
	if num != n {
		return CloseGoingAway
	}
	return nil
}
