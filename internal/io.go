package internal

import (
	"io"
	"unicode/utf8"
)

// ReadN 精准地读取len(data)个字节, 否则返回错误
func ReadN(reader io.Reader, data []byte) error {
	_, err := io.ReadFull(reader, data)
	return err
}

func WriteN(writer io.Writer, content []byte) error {
	_, err := writer.Write(content)
	return err
}

func CheckEncoding(opcode uint8, payload []byte) bool {
	switch opcode {
	case 1, 8:
		return utf8.Valid(payload)
	default:
		return true
	}
}

type Payload interface {
	io.WriterTo
	Len() int
	CheckEncoding(enabled bool, opcode uint8) bool
}

type Buffers [][]byte

func (b Buffers) CheckEncoding(enabled bool, opcode uint8) bool {
	if enabled {
		for i, _ := range b {
			if !CheckEncoding(opcode, b[i]) {
				return false
			}
		}
	}
	return true
}

func (b Buffers) Len() int {
	var sum = 0
	for i, _ := range b {
		sum += len(b[i])
	}
	return sum
}

// WriteTo 可重复写
func (b Buffers) WriteTo(w io.Writer) (int64, error) {
	var n = 0
	for i, _ := range b {
		x, err := w.Write(b[i])
		n += x
		if err != nil {
			return int64(n), err
		}
	}
	return int64(n), nil
}

type Bytes []byte

func (b Bytes) CheckEncoding(enabled bool, opcode uint8) bool {
	if enabled {
		return CheckEncoding(opcode, b)
	}
	return true
}

func (b Bytes) Len() int {
	return len(b)
}

// WriteTo 可重复写
func (b Bytes) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b)
	return int64(n), err
}
