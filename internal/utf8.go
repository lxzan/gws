// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// validUTF8 is a backport of unicode/utf8.Valid from golang/go commit
// 7b9de66 ("unicode/utf8: skip ahead during ascii runs in Valid/ValidString",
// released with Go 1.26). The utf8.Valid shipped with this module's Go 1.20
// toolchain floor leaves its ASCII fast path for good at the first non-ASCII
// byte and processes the rest of the input byte by byte. This variant
// re-enters the ASCII fast path after every rune.

package internal

const (
	runeSelf = 0x80

	// The default lowest and highest continuation byte.
	locb = 0b10000000
	hicb = 0b10111111

	// These names of these constants are chosen to give nice alignment in the
	// table below. The first nibble is an index into acceptRanges or F for
	// special one-byte cases. The second nibble is the Rune length or the
	// Status for the special one-byte case.
	xx = 0xF1 // invalid: size 1
	as = 0xF0 // ASCII: size 1
	s1 = 0x02 // accept 0, size 2
	s2 = 0x13 // accept 1, size 3
	s3 = 0x03 // accept 0, size 3
	s4 = 0x23 // accept 2, size 3
	s5 = 0x34 // accept 3, size 4
	s6 = 0x04 // accept 0, size 4
	s7 = 0x44 // accept 4, size 4
)

// utf8First is information about the first byte in a UTF-8 sequence.
var utf8First = [256]uint8{
	//   1   2   3   4   5   6   7   8   9   A   B   C   D   E   F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x00-0x0F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x10-0x1F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x20-0x2F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x30-0x3F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x40-0x4F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x50-0x5F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x60-0x6F
	as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, as, // 0x70-0x7F
	//   1   2   3   4   5   6   7   8   9   A   B   C   D   E   F
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0x80-0x8F
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0x90-0x9F
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0xA0-0xAF
	xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0xB0-0xBF
	xx, xx, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, // 0xC0-0xCF
	s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, s1, // 0xD0-0xDF
	s2, s3, s3, s3, s3, s3, s3, s3, s3, s3, s3, s3, s3, s4, s3, s3, // 0xE0-0xEF
	s5, s6, s6, s6, s7, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, xx, // 0xF0-0xFF
}

// utf8AcceptRange gives the range of valid values for the second byte in a
// UTF-8 sequence.
type utf8AcceptRange struct {
	lo uint8 // lowest value for second byte.
	hi uint8 // highest value for second byte.
}

// utf8AcceptRanges has size 16 to avoid bounds checks in the code that uses it.
var utf8AcceptRanges = [16]utf8AcceptRange{
	0: {locb, hicb},
	1: {0xA0, hicb},
	2: {locb, 0x9F},
	3: {0x90, hicb},
	4: {locb, 0x8F},
}

//nolint:mnd // bit-width constant inherent to pointer-size detection
const utf8PtrSize = 4 << (^uintptr(0) >> 63)

//nolint:mnd // bit-width constant inherent to pointer-size detection
const utf8HiBits = 0x8080808080808080 >> (64 - 8*utf8PtrSize)

//nolint:mnd // bit-shift constants inherent to word-size branching
func utf8Word(s []byte) uintptr {
	if utf8PtrSize == 4 {
		return uintptr(s[0]) | uintptr(s[1])<<8 | uintptr(s[2])<<16 | uintptr(s[3])<<24
	}
	return uintptr(uint64(s[0]) | uint64(s[1])<<8 | uint64(s[2])<<16 | uint64(s[3])<<24 | uint64(s[4])<<32 | uint64(s[5])<<40 | uint64(s[6])<<48 | uint64(s[7])<<56)
}

// validUTF8 reports whether p consists entirely of valid UTF-8-encoded runes.
//
//nolint:cyclop,mnd // backport of stdlib unicode/utf8.Valid; algorithm is fixed by upstream
func validUTF8(p []byte) bool {
	// This optimization avoids the need to recompute the capacity
	// when generating code for slicing p.
	p = p[:len(p):len(p)]

	for len(p) > 0 {
		p0 := p[0]
		if p0 < runeSelf {
			p = p[1:]
			// If there's one ASCII byte, there are probably more.
			// Advance quickly through ASCII-only data.
			// Note: using > instead of >= here is intentional. That avoids
			// needing pointing-past-the-end fixup on the slice operations.
			if len(p) > utf8PtrSize && utf8Word(p)&utf8HiBits == 0 {
				p = p[utf8PtrSize:]
				if len(p) > 2*utf8PtrSize && (utf8Word(p)|utf8Word(p[utf8PtrSize:]))&utf8HiBits == 0 {
					p = p[2*utf8PtrSize:]
					for len(p) > 4*utf8PtrSize && ((utf8Word(p)|utf8Word(p[utf8PtrSize:]))|(utf8Word(p[2*utf8PtrSize:])|utf8Word(p[3*utf8PtrSize:])))&utf8HiBits == 0 {
						p = p[4*utf8PtrSize:]
					}
				}
			}
			continue
		}
		x := utf8First[p0]
		size := int(x & 7)
		accept := utf8AcceptRanges[x>>4]
		switch size {
		case 2:
			if len(p) < 2 || p[1] < accept.lo || accept.hi < p[1] {
				return false
			}
			p = p[2:]
		case 3:
			if len(p) < 3 || p[1] < accept.lo || accept.hi < p[1] || p[2] < locb || hicb < p[2] {
				return false
			}
			p = p[3:]
		case 4:
			if len(p) < 4 || p[1] < accept.lo || accept.hi < p[1] || p[2] < locb || hicb < p[2] || p[3] < locb || hicb < p[3] {
				return false
			}
			p = p[4:]
		default:
			return false // illegal starter byte
		}
	}
	return true
}
