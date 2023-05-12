package internal

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
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
func MethodExists(in interface{}, method string) (reflect.Value, bool) {
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

func FNV64(s string) uint64 {
	var h = uint64(offset64)
	for _, b := range s {
		h *= prime64
		h ^= uint64(b)
	}
	return h
}

func NewBufferWithCap(n uint8) *bytes.Buffer {
	if n == 0 {
		return bytes.NewBuffer(nil)
	}
	return bytes.NewBuffer(make([]byte, 0, n))
}

/*
IO Utils
ReadN
WriteN
CopyN
*/
func CheckIOError(expectN, realN int, err error) error {
	if err != nil {
		return NewError(CloseInternalServerErr, err)
	}
	if realN != expectN {
		return NewError(CloseInternalServerErr, ErrUnexpectedContentLength)
	}
	return nil
}

func ReadN(reader io.Reader, data []byte, n int) error {
	if n == 0 {
		return nil
	}
	num, err := io.ReadFull(reader, data)
	return CheckIOError(n, num, err)
}

func WriteN(writer io.Writer, content []byte, n int) error {
	if n == 0 {
		return nil
	}
	num, err := writer.Write(content)
	return CheckIOError(n, num, err)
}

func CopyN(dst io.Writer, src io.Reader, n int64) error {
	if n == 0 {
		return nil
	}
	num, err := io.CopyN(dst, src, n)
	return CheckIOError(int(n), int(num), err)
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

func SelectInt[T Integer](ok bool, a, b T) T {
	if ok {
		return a
	}
	return b
}

func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsNil()
}

func ToBinaryNumber[T Integer](n T) T {
	var x T = 1
	for x < n {
		x *= 2
	}
	return x
}
