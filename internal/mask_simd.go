//go:build goexperiment.simd

package internal

import (
	"encoding/binary"
	"simd/archsimd"
)

// MaskXOR 计算掩码
// MaskXOR calculates the mask
var MaskXOR func(b []byte, key []byte) = MaskXOR_Scalar

func init() {
	if archsimd.X86.AVX2() {
		MaskXOR = MaskXOR_SIMD256
	} else if archsimd.X86.AVX() {
		MaskXOR = MaskXOR_SIMD128
	}
}

// MaskXOR_SIMD128 applies the WebSocket masking key using 128-bit SIMD.
//
// Large buffers use vector XOR for better throughput.
// Small buffers fall back to the scalar path to avoid SIMD setup cost.
func MaskXOR_SIMD128(b []byte, key []byte) {
	key32 := binary.LittleEndian.Uint32(key)
	key64 := uint64(key32)<<32 | uint64(key32)

	// simd initialization is expensive, so we only use it for large buffers
	if len(b) >= 512 {
		var key128Bytes [16]byte
		binary.LittleEndian.PutUint64(key128Bytes[0:8], key64)
		binary.LittleEndian.PutUint64(key128Bytes[8:16], key64)

		key128 := archsimd.LoadUint8x16(&key128Bytes)

		for len(b) >= 128 {
			v := archsimd.LoadUint8x16Slice(b[0:16]).Xor(key128)
			v.StoreSlice(b[0:16])

			v = archsimd.LoadUint8x16Slice(b[16:32]).Xor(key128)
			v.StoreSlice(b[16:32])

			v = archsimd.LoadUint8x16Slice(b[32:48]).Xor(key128)
			v.StoreSlice(b[32:48])

			v = archsimd.LoadUint8x16Slice(b[48:64]).Xor(key128)
			v.StoreSlice(b[48:64])

			v = archsimd.LoadUint8x16Slice(b[64:80]).Xor(key128)
			v.StoreSlice(b[64:80])

			v = archsimd.LoadUint8x16Slice(b[80:96]).Xor(key128)
			v.StoreSlice(b[80:96])

			v = archsimd.LoadUint8x16Slice(b[96:112]).Xor(key128)
			v.StoreSlice(b[96:112])

			v = archsimd.LoadUint8x16Slice(b[112:128]).Xor(key128)
			v.StoreSlice(b[112:128])

			b = b[128:]
		}
		if len(b) == 0 {
			return
		}
	}

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

// MaskXOR_SIMD256 applies the WebSocket masking key using 256-bit AVX2.
//
// Large buffers use AVX2 vector XOR for higher throughput.
// Small buffers fall back to the scalar path.
func MaskXOR_SIMD256(b []byte, key []byte) {
	key32 := binary.LittleEndian.Uint32(key)
	key64 := uint64(key32)<<32 | uint64(key32)

	// simd initialization is expensive, so we only use it for large buffers
	if len(b) >= 512 {
		var key256Bytes [32]byte
		binary.LittleEndian.PutUint64(key256Bytes[0:8], key64)
		binary.LittleEndian.PutUint64(key256Bytes[8:16], key64)
		binary.LittleEndian.PutUint64(key256Bytes[16:24], key64)
		binary.LittleEndian.PutUint64(key256Bytes[24:32], key64)

		key256 := archsimd.LoadUint8x32(&key256Bytes)

		for len(b) >= 128 {
			v := archsimd.LoadUint8x32Slice(b[0:32]).Xor(key256)
			v.StoreSlice(b[0:32])

			v = archsimd.LoadUint8x32Slice(b[32:64]).Xor(key256)
			v.StoreSlice(b[32:64])

			v = archsimd.LoadUint8x32Slice(b[64:96]).Xor(key256)
			v.StoreSlice(b[64:96])

			v = archsimd.LoadUint8x32Slice(b[96:128]).Xor(key256)
			v.StoreSlice(b[96:128])

			b = b[128:]
		}

		// perform VZEROUPPER to avoid AVX-SSE transition penalty on next scalar code
		// without this the performance can tank drastically on mixed workload.
		archsimd.ClearAVXUpperBits()

		if len(b) == 0 {
			return
		}
	}

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

// MaskXOR_Scalar applies the WebSocket masking key using scalar operations.
func MaskXOR_Scalar(b []byte, key []byte) {
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
