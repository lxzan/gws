package internal

import (
	"crypto/sha1"
	"encoding/base64"
	"math/rand"
	"net/http"
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
