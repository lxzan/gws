package internal

import (
	"bytes"
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

type Payload struct {
	data    []byte
	buffers [][]byte
}

func Bytes(p []byte) Payload {
	return Payload{data: p}
}

func Buffers(p [][]byte) Payload {
	return Payload{buffers: p}
}

func (p Payload) WriteToBuffer(b *bytes.Buffer) {
	if p.buffers == nil {
		b.Write(p.data)
		return
	}
	for i := range p.buffers {
		b.Write(p.buffers[i])
	}
}

func (p Payload) CheckEncoding(enabled bool, opcode uint8) bool {
	if !enabled || (opcode != 1 && opcode != 8) {
		return true
	}
	if p.buffers == nil {
		return utf8.Valid(p.data)
	}
	return true
}

func (p Payload) Len() int {
	if p.buffers == nil {
		return len(p.data)
	}
	var sum = 0
	for i := range p.buffers {
		sum += len(p.buffers[i])
	}
	return sum
}

// WriteTo 可重复写
func (p Payload) WriteTo(w io.Writer) (int64, error) {
	if p.buffers == nil {
		n, err := w.Write(p.data)
		return int64(n), err
	}
	var n = 0
	for i := range p.buffers {
		x, err := w.Write(p.buffers[i])
		n += x
		if err != nil {
			return int64(n), err
		}
	}
	return int64(n), nil
}
