package internal

import (
	"io"
	"unicode/utf8"
)

// ReadN 精准地读取 len(data) 个字节, 否则返回错误
func ReadN(reader io.Reader, data []byte) error {
	_, err := io.ReadFull(reader, data)
	return err
}

// WriteN 将 content 写入 writer 中
func WriteN(writer io.Writer, content []byte) error {
	_, err := writer.Write(content)
	return err
}

// CheckEncoding 检查 payload 的编码是否有效
func CheckEncoding(enabled bool, opcode uint8, payload []byte) bool {
	if enabled && (opcode == 1 || opcode == 8) {
		return validUTF8(payload)
	}
	return true
}

type Payload interface {
	io.WriterTo
	Len() int
	CheckEncoding(enabled bool, opcode uint8) bool
}

type Buffers [][]byte

// CheckEncoding 按拼接语义校验 UTF-8, 跨切片截断的多字节序列可正确续判
func (b Buffers) CheckEncoding(enabled bool, opcode uint8) bool {
	if !enabled || (opcode != 1 && opcode != 8) {
		return true
	}
	var pending [utf8.UTFMax]byte
	var n int
	for i := range b {
		var p = b[i]
		if n > 0 {
			var ok bool
			p, n, ok = resolvePending(p, pending[:], n)
			if !ok {
				return false
			}
			if p == nil {
				continue
			}
		}
		if validUTF8(p) {
			continue
		}
		var ok bool
		n, ok = checkTrailingBytes(p, pending[:])
		if !ok {
			return false
		}
	}
	return n == 0
}

// resolvePending 处理上一切片残留的 pending 字节与当前切片 p 的拼接续判。
// 返回剩余未消费的 p、更新后的 pending 长度 n、以及是否合法。
// 当 p 被完全消费（仍凑不够一个完整 rune）时 rest 返回 nil。
func resolvePending(p []byte, pending []byte, n int) (rest []byte, newN int, ok bool) {
	var k = utf8.UTFMax - n
	k = min(k, len(p))
	copy(pending[n:], p[:k])
	if r, m := utf8.DecodeRune(pending[:n+k]); r != utf8.RuneError || m != 1 {
		return p[m-n:], 0, true
	} else if k == utf8.UTFMax-n {
		return nil, 0, false
	} else {
		return nil, n + k, true
	}
}

// checkTrailingBytes 检查 p 尾部是否为不完整的多字节序列起始。
// 若尾部截断的序列合法（可续接到下一切片），将其拷贝到 pending 并返回新的 pending 长度。
func checkTrailingBytes(p []byte, pending []byte) (newN int, ok bool) {
	var j = len(p) - 1
	for j >= 0 && j > len(p)-utf8.UTFMax && p[j]&0xC0 == 0x80 {
		j--
	}
	if j < 0 || p[j] < 0xC0 {
		return 0, false
	}
	if _, m := utf8.DecodeRune(p[j:]); m > 1 || !validUTF8(p[:j]) {
		return 0, false
	}
	return copy(pending, p[j:]), true
}

func (b Buffers) Len() int {
	var sum = 0
	for i := range b {
		sum += len(b[i])
	}
	return sum
}

// WriteTo 可重复写
func (b Buffers) WriteTo(w io.Writer) (int64, error) {
	var n = 0
	for i := range b {
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
