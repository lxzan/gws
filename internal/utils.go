package internal

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"math/rand"
	"net/http"
	"reflect"
	"unsafe"
)

const (
	prime64  = 1099511628211
	offset64 = 14695981039346656037
)

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
	n := rand.Uint32()
	return [4]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}
}

func CloneHeader(h http.Header) http.Header {
	header := http.Header{}
	for k, v := range h {
		header[k] = v
	}
	return header
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
*/
func checkIOError(expectN, realN int, err error) error {
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
	return checkIOError(n, num, err)
}

func WriteN(writer io.Writer, content []byte, n int) error {
	if n == 0 {
		return nil
	}
	num, err := writer.Write(content)
	return checkIOError(n, num, err)
}

func CopyN(dst io.Writer, src io.Reader, n int64) error {
	if n == 0 {
		return nil
	}
	num, err := io.CopyN(dst, src, n)
	return checkIOError(int(n), int(num), err)
}
