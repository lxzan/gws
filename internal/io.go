package internal

import (
	"io"
	"unicode/utf8"
)

// ReadN 精准地读取 len(data) 个字节, 否则返回错误
// reads exactly len(data) bytes, otherwise returns an error
func ReadN(reader io.Reader, data []byte) error {
	_, err := io.ReadFull(reader, data)
	return err
}

// WriteN 将 content 写入 writer 中
// writes the content to the writer
func WriteN(writer io.Writer, content []byte) error {
	_, err := writer.Write(content)
	return err
}

// CheckEncoding 检查 payload 的编码是否有效
// checks if the encoding of the payload is valid
func CheckEncoding(enabled bool, opcode uint8, payload []byte) bool {
	if enabled && (opcode == 1 || opcode == 8) {
		return utf8.Valid(payload)
	}
	return true
}

type Payload interface {
	io.WriterTo
	Len() int
	CheckEncoding(enabled bool, opcode uint8) bool
}

type Buffers [][]byte

func (b Buffers) CheckEncoding(enabled bool, opcode uint8) bool {
	for i, _ := range b {
		if !CheckEncoding(enabled, opcode, b[i]) {
			return false
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
	return CheckEncoding(enabled, opcode, b)
}

func (b Bytes) Len() int {
	return len(b)
}

// WriteTo 可重复写
func (b Bytes) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b)
	return int64(n), err
}
