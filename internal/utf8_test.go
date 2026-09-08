// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Differential tests for validUTF8, the backport of unicode/utf8.Valid from
// golang/go commit 7b9de66. The invalid-case matrix hand-adapts the canonical
// illegal-UTF-8 vectors also exercised by the Go standard library tests.

package internal

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

var utf8TestRunePools = [][]rune{
	{'a', 'Z', '0', ' ', '\n', '\r', '\t', '\x01', '\x7f', '"', '\'', '/', '{'},
	{'中', '文', '汉', '字', '永', '和', '平', '道'},
	{'😀', '🚀', '🌍', '🎉', '🐼', '🦄'},
	{'α', 'β', 'Ω', 'Ж', 'д', '€', '￡', '￠'},
}

func utf8TestASCIIRun(rng *rand.Rand, maxLen int) []byte {
	var n = rng.Intn(maxLen+1) + 1
	var p = make([]byte, n)
	for i := range p {
		p[i] = byte(rng.Intn(128))
	}
	return p
}

func utf8TestRuneSequence(rng *rand.Rand, maxRunes int) []byte {
	var buf []byte
	var n = rng.Intn(maxRunes+1) + 1
	var scratch [utf8.UTFMax]byte
	for i := 0; i < n; i++ {
		var pool = utf8TestRunePools[rng.Intn(len(utf8TestRunePools))]
		var r = pool[rng.Intn(len(pool))]
		buf = append(buf, scratch[:utf8.EncodeRune(scratch[:], r)]...)
	}
	return buf
}

func utf8CaseRandom(rng *rand.Rand) []byte {
	var p = make([]byte, rng.Intn(129))
	rng.Read(p)
	return p
}

func utf8CaseCorrupt(rng *rand.Rand) []byte {
	var p = utf8TestRuneSequence(rng, 96)
	if len(p) == 0 {
		return p
	}
	p[rng.Intn(len(p))] = byte(rng.Intn(256))
	return p
}

func utf8CaseTruncate(rng *rand.Rand) []byte {
	var p = utf8TestRuneSequence(rng, 96)
	return p[:rng.Intn(len(p)+1)]
}

func utf8CaseInsert(rng *rand.Rand) []byte {
	var p = utf8TestRuneSequence(rng, 64)
	var pos = rng.Intn(len(p) + 1)
	var out = make([]byte, 0, len(p)+1)
	out = append(out, p[:pos]...)
	out = append(out, byte(rng.Intn(256)))
	return append(out, p[pos:]...)
}

func utf8CaseMixed(rng *rand.Rand) []byte {
	var buf []byte
	var n = rng.Intn(97) + 1
	var scratch [utf8.UTFMax]byte
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			buf = append(buf, byte(rng.Intn(128)))
			continue
		}
		var pool = utf8TestRunePools[1+rng.Intn(len(utf8TestRunePools)-1)]
		var r = pool[rng.Intn(len(pool))]
		buf = append(buf, scratch[:utf8.EncodeRune(scratch[:], r)]...)
	}
	return buf
}

func utf8CaseConcat(rng *rand.Rand) []byte {
	var buf []byte
	for len(buf) < 128+rng.Intn(897) {
		buf = append(buf, utf8TestRuneSequence(rng, 32)...)
		buf = append(buf, utf8TestASCIIRun(rng, 64)...)
	}
	return buf
}

func utf8CaseOverlong(rng *rand.Rand) []byte {
	var buf = []byte{byte(0xC0 + rng.Intn(0x40))}
	var n = rng.Intn(4)
	for i := 0; i < n; i++ {
		if rng.Intn(2) == 0 {
			buf = append(buf, byte(0x80+rng.Intn(0x40)))
		} else {
			buf = append(buf, byte(rng.Intn(256)))
		}
	}
	if rng.Intn(2) == 0 {
		buf = append(buf, utf8TestRuneSequence(rng, 8)...)
	}
	return buf
}

func utf8CaseDefault(rng *rand.Rand) []byte {
	var buf = utf8TestASCIIRun(rng, 32)
	var pool = utf8TestRunePools[1+rng.Intn(len(utf8TestRunePools)-1)]
	var scratch [utf8.UTFMax]byte
	var r = pool[rng.Intn(len(pool))]
	buf = append(buf, scratch[:utf8.EncodeRune(scratch[:], r)]...)
	buf = append(buf, utf8TestASCIIRun(rng, 32)...)
	if rng.Intn(2) == 0 && len(buf) > 0 {
		buf[len(buf)-1-rng.Intn(len(buf))] ^= byte(1 << rng.Intn(8))
	}
	return buf
}

func utf8TestInputGroupA(rng *rand.Rand, c int) []byte {
	switch c {
	case 0:
		return utf8TestASCIIRun(rng, 256)
	case 1:
		return utf8TestRuneSequence(rng, 96)
	case 2:
		return utf8CaseRandom(rng)
	case 3:
		return utf8CaseCorrupt(rng)
	default:
		return utf8CaseTruncate(rng)
	}
}

func utf8TestInputGroupB(rng *rand.Rand, c int) []byte {
	switch c {
	case 0:
		return utf8CaseInsert(rng)
	case 1:
		return utf8CaseMixed(rng)
	case 2:
		return utf8CaseConcat(rng)
	case 3:
		return utf8CaseOverlong(rng)
	default:
		return utf8CaseDefault(rng)
	}
}

// utf8TestInput generates one fuzz input. Valid and invalid shapes are mixed
// deliberately: strategies 1/6/7 produce valid UTF-8, the others corrupt,
// truncate or synthesize bytes so that both outcomes are well represented.
func utf8TestInput(rng *rand.Rand, seq int) []byte {
	var c = seq % 10
	if c < 5 {
		return utf8TestInputGroupA(rng, c)
	}
	return utf8TestInputGroupB(rng, c-5)
}

func assertUTF8Agrees(t *testing.T, p []byte, want bool) {
	t.Helper()
	var got = validUTF8(p)
	var std = utf8.Valid(p)
	if got != want || std != want {
		t.Fatalf("input %q (%x): validUTF8=%v utf8.Valid=%v want=%v", p, p, got, std, want)
	}
}

func TestValidUTF8_DifferentialRandom(t *testing.T) {
	const cases = 120000
	var rng = rand.New(rand.NewSource(20260904))
	for i := 0; i < cases; i++ {
		var p = utf8TestInput(rng, i)
		var want = utf8.Valid(p)
		if got := validUTF8(p); got != want {
			t.Fatalf("case %d: validUTF8=%v utf8.Valid=%v input %q (%x)", i, got, want, p, p)
		}
	}
}

func TestValidUTF8_DifferentialLong(t *testing.T) {
	var rng = rand.New(rand.NewSource(7))
	for i := 0; i < 50; i++ {
		var buf []byte
		for len(buf) < 16*1024+rng.Intn(48*1024) {
			buf = append(buf, utf8TestRuneSequence(rng, 64)...)
			buf = append(buf, utf8TestASCIIRun(rng, 256)...)
		}
		assertUTF8Agrees(t, buf, true)
		var pos = rng.Intn(len(buf))
		var corrupted = append([]byte{}, buf...)
		corrupted[pos] = byte(rng.Intn(256))
		if got, want := validUTF8(corrupted), utf8.Valid(corrupted); got != want {
			t.Fatalf("long case %d corrupted@%d: validUTF8=%v utf8.Valid=%v", i, pos, got, want)
		}
		var cut = rng.Intn(len(buf))
		if got, want := validUTF8(buf[:cut]), utf8.Valid(buf[:cut]); got != want {
			t.Fatalf("long case %d cut@%d: validUTF8=%v utf8.Valid=%v", i, cut, got, want)
		}
	}
}

func TestValidUTF8_InvalidMatrix(t *testing.T) {
	var cn = []byte("中文")
	var zhong = []byte("中") // E4 B8 AD
	var emoji = []byte("😀") // F0 9F 98 80

	var tests = []struct {
		name string
		in   []byte
		want bool
	}{
		{"empty", nil, true},
		{"ascii", []byte("hello, world"), true},
		{"controls", []byte("\x00\x01\r\n\t\x7f"), true},
		{"cjk", cn, true},
		{"emoji", emoji, true},
		{"mixed", append(append([]byte("a "), cn...), append(emoji, 'b')...), true},
		{"u+0080", []byte{0xC2, 0x80}, true},
		{"u+07ff", []byte{0xDF, 0xBF}, true},
		{"u+0800", []byte{0xE0, 0xA0, 0x80}, true},
		{"u+d7ff", []byte{0xED, 0x9F, 0xBF}, true},
		{"u+e000", []byte{0xEE, 0x80, 0x80}, true},
		{"u+ffff", []byte{0xEF, 0xBF, 0xBF}, true},
		{"u+fffd", []byte{0xEF, 0xBF, 0xBD}, true},
		{"u+10000", []byte{0xF0, 0x90, 0x80, 0x80}, true},
		{"u+10ffff", []byte{0xF4, 0x8F, 0xBF, 0xBF}, true},

		{"trunc 3-byte at 1", zhong[:1], false},
		{"trunc 3-byte at 2", zhong[:2], false},
		{"trunc 4-byte at 1", emoji[:1], false},
		{"trunc 4-byte at 2", emoji[:2], false},
		{"trunc 4-byte at 3", emoji[:3], false},
		{"ascii then trunc", append([]byte{'a'}, zhong[:2]...), false},
		{"trunc then ascii", append(append([]byte{}, zhong[:2]...), 'a'), false},
		{"valid then trunc", append(append([]byte{}, cn...), emoji[:3]...), false},

		{"overlong c0 80", []byte{0xC0, 0x80}, false},
		{"overlong c1 bf", []byte{0xC1, 0xBF}, false},
		{"overlong e0 80 80", []byte{0xE0, 0x80, 0x80}, false},
		{"overlong e0 9f bf", []byte{0xE0, 0x9F, 0xBF}, false},
		{"overlong f0 80 80 80", []byte{0xF0, 0x80, 0x80, 0x80}, false},
		{"overlong f0 8f bf-bf", []byte{0xF0, 0x8F, 0xBF, 0xBF}, false},

		{"surrogate d800", []byte{0xED, 0xA0, 0x80}, false},
		{"surrogate mid", []byte{0xED, 0xAD, 0xBF}, false},
		{"surrogate dfff", []byte{0xED, 0xBF, 0xBF}, false},
		{"surrogate pair", []byte{0xED, 0xA0, 0x80, 0xED, 0xBF, 0xBF}, false},

		{"above u+10ffff", []byte{0xF4, 0x90, 0x80, 0x80}, false},
		{"starter f5", []byte{0xF5, 0x80, 0x80, 0x80}, false},
		{"starter f5 max", []byte{0xF5, 0xBF, 0xBF, 0xBF}, false},
		{"starter f6", []byte{0xF6, 0x80, 0x80, 0x80}, false},
		{"starter f7", []byte{0xF7, 0xBF, 0xBF, 0xBF}, false},
		{"starter f8 5-byte", []byte{0xF8, 0x88, 0x80, 0x80, 0x80}, false},
		{"starter fc 6-byte", []byte{0xFC, 0x84, 0x80, 0x80, 0x80, 0x80}, false},
		{"byte fe", []byte{0xFE}, false},
		{"byte ff", []byte{0xFF}, false},
		{"fe ff", []byte{0xFE, 0xFF}, false},
		{"ff in context", append(append([]byte{}, cn...), append([]byte{0xFF}, cn...)...), false},

		{"lone continuation 80", []byte{0x80}, false},
		{"lone continuation bf", []byte{0xBF}, false},
		{"four continuations", []byte{0x80, 0x80, 0x80, 0x80}, false},
		{"ascii then continuation", []byte{'a', 0x80}, false},
		{"continuation then ascii", []byte{0x80, 'a'}, false},
		{"extra continuation after rune", append(append([]byte{}, zhong...), 0xBF), false},

		{"bad 2nd byte 00", []byte{0xE4, 0x00, 0xAD}, false},
		{"bad 3rd byte 00", []byte{0xE4, 0xB8, 0x00}, false},
		{"bad 4th byte 00", []byte{0xF0, 0x9F, 0x98, 0x00}, false},
		{"bad 2nd byte in 4-byte", []byte{0xF0, 0x90, 0x00, 0x80}, false},
		{"bad 2nd byte c2 00", []byte{0xC2, 0x00}, false},
		{"starter as 2nd byte", []byte{0xE4, 0xC0, 0xAD}, false},
		{"overlong nested in text", append([]byte("a"), append([]byte{0xC0, 0x80}, 'a')...), false},
		{"surrogate in text", append([]byte("hello"), []byte{0xED, 0xA0, 0x80}...), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUTF8Agrees(t, tt.in, tt.want)
		})
	}
}

func TestValidUTF8_BoundaryLengths(t *testing.T) {
	var zhong = []byte("中")
	var emoji = []byte("😀")
	for n := 0; n <= 40; n++ {
		var ascii = bytes.Repeat([]byte{'a'}, n)
		assertUTF8Agrees(t, ascii, true)
		assertUTF8Agrees(t, append(append([]byte{}, ascii...), zhong...), true)
		assertUTF8Agrees(t, append(append([]byte{}, zhong...), ascii...), true)
		assertUTF8Agrees(t, append(append([]byte{}, ascii...), emoji...), true)
		for k := 1; k < len(zhong); k++ {
			assertUTF8Agrees(t, append(append([]byte{}, ascii...), zhong[:len(zhong)-k]...), false)
		}
		for k := 1; k < len(emoji); k++ {
			assertUTF8Agrees(t, append(append([]byte{}, ascii...), emoji[:len(emoji)-k]...), false)
		}
	}
}

func TestValidUTF8_ZeroAlloc(t *testing.T) {
	var rng = rand.New(rand.NewSource(3))
	var data []byte
	for len(data) < 64*1024 {
		data = append(data, utf8TestRuneSequence(rng, 64)...)
		data = append(data, utf8TestASCIIRun(rng, 64)...)
	}
	var ok bool
	var allocs = testing.AllocsPerRun(100, func() {
		ok = validUTF8(data)
	})
	assert.True(t, ok)
	assert.Zero(t, allocs)
}

// utf8TestSplit cuts p into 1-5 slices. Half of the cut points are snapped to
// continuation bytes so that multi-byte runes are split across slice borders,
// mirroring the Buffers.CheckEncoding consumption pattern.
func utf8TestSplit(rng *rand.Rand, p []byte) [][]byte {
	if len(p) < 2 {
		return [][]byte{p}
	}
	var nCuts = rng.Intn(5)
	var cuts = make([]int, 0, nCuts)
	for i := 0; i < nCuts; i++ {
		var pos = 1 + rng.Intn(len(p)-1)
		if rng.Intn(2) == 0 {
			for j := pos; j < len(p) && j < pos+utf8.UTFMax; j++ {
				if p[j]&0xC0 == 0x80 {
					pos = j
					break
				}
			}
		}
		cuts = append(cuts, pos)
	}
	sort.Ints(cuts)
	var result = make([][]byte, 0, len(cuts)+1)
	var start int
	for _, c := range cuts {
		result = append(result, p[start:c])
		start = c
	}
	return append(result, p[start:])
}

func TestBuffers_CheckEncoding_DifferentialRandom(t *testing.T) {
	const cases = 20000
	var rng = rand.New(rand.NewSource(9))
	for i := 0; i < cases; i++ {
		var p = utf8TestInput(rng, i)
		var slices = utf8TestSplit(rng, p)
		var want = utf8.Valid(bytes.Join(slices, nil))
		var opcode uint8 = 1
		if rng.Intn(4) == 0 {
			opcode = 8
		}
		if got := Buffers(slices).CheckEncoding(true, opcode); got != want {
			t.Fatalf("case %d: CheckEncoding=%v utf8.Valid(concat)=%v slices=%q", i, got, want, slices)
		}
	}
}
