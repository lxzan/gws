//go:build !goexperiment.simd

package internal

import "encoding/binary"

// MaskXOR 计算掩码
// MaskXOR calculates the mask
func MaskXOR(b []byte, key []byte) {
	key32 := binary.LittleEndian.Uint32(key)
	key64 := uint64(key32)<<32 | uint64(key32)

	for len(b) >= 64 {
		v := binary.LittleEndian.Uint64(b[0:8])
		binary.LittleEndian.PutUint64(b[0:8], v^key64)

		v = binary.LittleEndian.Uint64(b[8:16])
		binary.LittleEndian.PutUint64(b[8:16], v^key64)

		v = binary.LittleEndian.Uint64(b[16:24])
		binary.LittleEndian.PutUint64(b[16:24], v^key64)

		v = binary.LittleEndian.Uint64(b[24:32])
		binary.LittleEndian.PutUint64(b[24:32], v^key64)

		v = binary.LittleEndian.Uint64(b[32:40])
		binary.LittleEndian.PutUint64(b[32:40], v^key64)

		v = binary.LittleEndian.Uint64(b[40:48])
		binary.LittleEndian.PutUint64(b[40:48], v^key64)

		v = binary.LittleEndian.Uint64(b[48:56])
		binary.LittleEndian.PutUint64(b[48:56], v^key64)

		v = binary.LittleEndian.Uint64(b[56:64])
		binary.LittleEndian.PutUint64(b[56:64], v^key64)

		b = b[64:]
	}
	if len(b) == 0 {
		return
	}

	if len(b) >= 32 {
		v := binary.LittleEndian.Uint64(b[0:8])
		binary.LittleEndian.PutUint64(b[0:8], v^key64)

		v = binary.LittleEndian.Uint64(b[8:16])
		binary.LittleEndian.PutUint64(b[8:16], v^key64)

		v = binary.LittleEndian.Uint64(b[16:24])
		binary.LittleEndian.PutUint64(b[16:24], v^key64)

		v = binary.LittleEndian.Uint64(b[24:32])
		binary.LittleEndian.PutUint64(b[24:32], v^key64)

		b = b[32:]
		if len(b) == 0 {
			return
		}
	}

	if len(b) >= 16 {
		v := binary.LittleEndian.Uint64(b[0:8])
		binary.LittleEndian.PutUint64(b[0:8], v^key64)

		v = binary.LittleEndian.Uint64(b[8:16])
		binary.LittleEndian.PutUint64(b[8:16], v^key64)

		b = b[16:]
		if len(b) == 0 {
			return
		}
	}

	if len(b) >= 8 {
		v := binary.LittleEndian.Uint64(b[0:8])
		binary.LittleEndian.PutUint64(b[0:8], v^key64)

		b = b[8:]
		if len(b) == 0 {
			return
		}
	}

	if len(b) >= 4 {
		v := binary.LittleEndian.Uint32(b[0:4])
		binary.LittleEndian.PutUint32(b[0:4], v^key32)

		b = b[4:]
		if len(b) == 0 {
			return
		}
	}

	for i := range b {
		b[i] ^= key[i&3]
	}
}
