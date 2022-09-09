package internal

import (
	"math/rand"
	"time"
)

var R = rand.New(rand.NewSource(time.Now().UnixNano()))

type RandomString string

const (
	AlphabetNumeric RandomString = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Numeric         RandomString = "0123456789"
)

func (c RandomString) Generate(n int) []byte {
	var b = make([]byte, n)
	var length = len(c)
	for i := 0; i < n; i++ {
		var idx = R.Intn(length)
		b[i] = c[idx]
	}
	return b
}
