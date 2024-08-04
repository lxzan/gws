package internal

import (
	"io"
	"unicode/utf8"
)

// ReadN 精准地读取 len(data) 个字节, 否则返回错误
// ReadN reads exactly len(data) bytes, otherwise returns an error
func ReadN(reader io.Reader, data []byte) error {
	// 使用 io.ReadFull 函数从 reader 中读取 len(data) 个字节
	// Use io.ReadFull to read len(data) bytes from the reader
	_, err := io.ReadFull(reader, data)

	// 返回读取过程中遇到的错误
	// Return any error encountered during reading
	return err
}

// WriteN 将 content 写入 writer 中, 否则返回错误
// WriteN writes the content to the writer, otherwise returns an error
func WriteN(writer io.Writer, content []byte) error {
	// 使用 writer.Write 函数将 content 写入 writer 中
	// Use writer.Write to write the content to the writer
	_, err := writer.Write(content)

	// 返回写入过程中遇到的错误
	// Return any error encountered during writing
	return err
}

// CheckEncoding 检查 payload 的编码是否有效
// CheckEncoding checks if the encoding of the payload is valid
func CheckEncoding(opcode uint8, payload []byte) bool {
	// 根据 opcode 的值进行不同的处理
	// Handle different cases based on the value of opcode
	switch opcode {
	// 如果 opcode 是 1 或 8, 检查 payload 是否是有效的 UTF-8 编码
	// If opcode is 1 or 8, check if the payload is valid UTF-8
	case 1, 8:
		return utf8.Valid(payload)

	// 对于其他 opcode, 始终返回 true
	// For other opcodes, always return true
	default:
		return true
	}
}

// Payload 接口定义了处理负载数据的方法
// Payload interface defines methods for handling payload data
type Payload interface {
	// WriterTo 接口用于将数据写入 io.Writer
	// WriterTo interface is used to write data to an io.Writer
	io.WriterTo

	// Len 返回负载数据的长度
	// Len returns the length of the payload data
	Len() int

	// CheckEncoding 检查负载数据的编码是否有效
	// CheckEncoding checks if the encoding of the payload data is valid
	CheckEncoding(enabled bool, opcode uint8) bool
}

// Buffers 类型定义为一个二维字节切片
// Buffers type is defined as a slice of byte slices
type Buffers [][]byte

// CheckEncoding 检查每个缓冲区的编码是否有效
// CheckEncoding checks if the encoding of each buffer is valid
func (b Buffers) CheckEncoding(enabled bool, opcode uint8) bool {
	// 如果启用了编码检查
	// If encoding check is enabled
	if enabled {
		// 遍历每个缓冲区
		// Iterate over each buffer
		for i, _ := range b {
			// 如果任意一个缓冲区的编码无效，返回 false
			// If any buffer's encoding is invalid, return false
			if !CheckEncoding(opcode, b[i]) {
				return false
			}
		}
	}

	// 如果所有缓冲区的编码都有效，返回 true
	// If all buffers' encodings are valid, return true
	return true
}

// Len 返回所有缓冲区的总长度
// Len returns the total length of all buffers
func (b Buffers) Len() int {
	// 初始化总长度为 0
	// Initialize total length to 0
	var sum = 0

	// 遍历每个缓冲区
	// Iterate over each buffer
	for i, _ := range b {
		// 累加每个缓冲区的长度
		// Accumulate the length of each buffer
		sum += len(b[i])
	}

	// 返回总长度
	// Return the total length
	return sum
}

// WriteTo 将所有缓冲区的数据写入指定的 io.Writer
// WriteTo writes the data of all buffers to the specified io.Writer
func (b Buffers) WriteTo(w io.Writer) (int64, error) {
	// 初始化写入的总字节数为 0
	// Initialize the total number of bytes written to 0
	var n = 0

	// 遍历每个缓冲区
	// Iterate over each buffer
	for i, _ := range b {
		// 将当前缓冲区的数据写入 io.Writer
		// Write the current buffer's data to the io.Writer
		x, err := w.Write(b[i])

		// 累加写入的字节数
		// Accumulate the number of bytes written
		n += x

		// 如果写入过程中遇到错误，返回已写入的字节数和错误
		// If an error is encountered during writing, return the number of bytes written and the error
		if err != nil {
			return int64(n), err
		}
	}

	// 返回写入的总字节数和 nil 错误
	// Return the total number of bytes written and a nil error
	return int64(n), nil
}

// Bytes 类型定义为一个字节切片
// Bytes type is defined as a byte slice
type Bytes []byte

// CheckEncoding 检查字节切片的编码是否有效
// CheckEncoding checks if the encoding of the byte slice is valid
func (b Bytes) CheckEncoding(enabled bool, opcode uint8) bool {
	// 如果启用了编码检查
	// If encoding check is enabled
	if enabled {
		// 检查字节切片的编码是否有效
		// Check if the encoding of the byte slice is valid
		return CheckEncoding(opcode, b)
	}

	// 如果未启用编码检查，始终返回 true
	// If encoding check is not enabled, always return true
	return true
}

// Len 返回字节切片的长度
// Len returns the length of the byte slice
func (b Bytes) Len() int {
	return len(b)
}

// WriteTo 将字节切片的数据写入指定的 io.Writer
// WriteTo writes the data of the byte slice to the specified io.Writer
func (b Bytes) WriteTo(w io.Writer) (int64, error) {
	// 将字节切片的数据写入 io.Writer
	// Write the byte slice's data to the io.Writer
	n, err := w.Write(b)

	// 返回写入的字节数和可能的错误
	// Return the number of bytes written and any potential error
	return int64(n), err
}
