package internal

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"reflect"
	"strings"
	"unsafe"
)

const (
	prime64  = 1099511628211
	offset64 = 14695981039346656037
)

type Integer interface {
	int | int64 | int32 | uint | uint64 | uint32
}

func MaskByByte(content []byte, key []byte) {
	var n = len(content)
	for i := 0; i < n; i++ {
		var idx = i & 3
		content[i] ^= key[idx]
	}
}

func ComputeAcceptKey(challengeKey string) string {
	h := sha1.New()
	buf := []byte(challengeKey)
	buf = append(buf, MagicNumber...)
	h.Write(buf)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func NewMaskKey() [4]byte {
	n := AlphabetNumeric.Uint32()
	return [4]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}
}

// MethodExists
// if nil return false
func MethodExists(in any, method string) (reflect.Value, bool) {
	if in == nil || method == "" {
		return reflect.Value{}, false
	}
	p := reflect.TypeOf(in)
	if p.Kind() == reflect.Ptr {
		p = p.Elem()
	}
	if p.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	object := reflect.ValueOf(in)
	newMethod := object.MethodByName(method)
	if !newMethod.IsValid() {
		return reflect.Value{}, false
	}
	return newMethod, true
}

func StringToBytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

func FnvString(s string) uint64 {
	var h = uint64(offset64)
	for _, b := range s {
		h *= prime64
		h ^= uint64(b)
	}
	return h
}

func FnvNumber[T Integer](x T) uint64 {
	var h = uint64(offset64)
	h *= prime64
	h ^= uint64(x)
	return h
}

func MaskXOR(b []byte, key []byte) {
	var maskKey = binary.LittleEndian.Uint32(key)
	var key64 = uint64(maskKey)<<32 + uint64(maskKey)

	for len(b) >= 64 {
		v := binary.LittleEndian.Uint64(b)
		binary.LittleEndian.PutUint64(b, v^key64)
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

	for len(b) >= 8 {
		v := binary.LittleEndian.Uint64(b[:8])
		binary.LittleEndian.PutUint64(b[:8], v^key64)
		b = b[8:]
	}

	var n = len(b)
	for i := 0; i < n; i++ {
		idx := i & 3
		b[i] ^= key[idx]
	}
}

func InCollection(elem string, elems []string) bool {
	for _, item := range elems {
		if item == elem {
			return true
		}
	}
	return false
}

// GetIntersectionElem 获取两个数组交集中的一个元素
func GetIntersectionElem(a, b []string) string {
	for _, item := range a {
		if InCollection(item, b) {
			return item
		}
	}
	return ""
}

// Split 分割字符串(空值将会被过滤掉)
func Split(s string, sep string) []string {
	var list = strings.Split(s, sep)
	var j = 0
	for _, v := range list {
		if v = strings.TrimSpace(v); v != "" {
			list[j] = v
			j++
		}
	}
	return list[:j]
}

func HttpHeaderEqual(a, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b)
}

func HttpHeaderContains(a, b string) bool {
	return strings.Contains(strings.ToLower(a), strings.ToLower(b))
}

func SelectValue[T any](ok bool, a, b T) T {
	if ok {
		return a
	}
	return b
}

func ToBinaryNumber[T Integer](n T) T {
	var x T = 1
	for x < n {
		x *= 2
	}
	return x
}

func BinaryPow(n int) int {
	var ans = 1
	for i := 0; i < n; i++ {
		ans <<= 1
	}
	return ans
}

// BufferReset 重置buffer底层切片
// 修改后面的属性一定要加偏移量!!!
func BufferReset(b *bytes.Buffer, p []byte) { *(*[]byte)(unsafe.Pointer(b)) = p }

// IsZero 零值判断
func IsZero[T comparable](v T) bool {
	var zero T
	return v == zero
}

// WithDefault 如果原值为零值, 返回新值, 否则返回原值
func WithDefault[T comparable](rawValue, newValue T) T {
	if IsZero(rawValue) {
		return newValue
	}
	return rawValue
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func IsSameSlice[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
