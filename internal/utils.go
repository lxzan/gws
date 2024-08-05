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

// 定义一个常量 prime64，其值为 1099511628211
// Define a constant prime64, its value is 1099511628211
const prime64 = 1099511628211

// 定义一个常量 offset64，其值为 14695981039346656037
// Define a constant offset64, its value is 14695981039346656037
const offset64 = 14695981039346656037

// 定义一个接口 Integer，它可以是 int、int64、int32、uint、uint64 或 uint32 类型
// Define an interface Integer, it can be of type int, int64, int32, uint, uint64, or uint32
type Integer interface {
	int | int64 | int32 | uint | uint64 | uint32
}

// MaskByByte 是一个函数，接收两个字节切片参数 content 和 key，对 content 进行按位异或操作。
// MaskByByte is a function that takes two byte slice parameters, content and key, and performs bitwise XOR operation on content.
func MaskByByte(content []byte, key []byte) {
	// 获取 content 的长度，并赋值给 n
	// Get the length of content and assign it to n
	var n = len(content)

	// 遍历 content 中的每一个元素
	// Iterate over each element in content
	for i := 0; i < n; i++ {
		// 计算 i 与 3 的按位与运算结果，并赋值给 idx
		// Calculate the bitwise AND operation result of i and 3, and assign it to idx
		var idx = i & 3

		// 对 content[i] 和 key[idx] 进行按位异或操作，并将结果赋值给 content[i]
		// Perform bitwise XOR operation on content[i] and key[idx], and assign the result to content[i]
		content[i] ^= key[idx]
	}
}

// ComputeAcceptKey 是一个函数，接收一个字符串参数 challengeKey，计算其 SHA-1 哈希值，并返回其 Base64 编码。
// ComputeAcceptKey is a function that takes a string parameter challengeKey, calculates its SHA-1 hash, and returns its Base64 encoding.
func ComputeAcceptKey(challengeKey string) string {
	// 创建一个新的 SHA-1 哈希
	// Create a new SHA-1 hash
	h := sha1.New()

	// 将 challengeKey 转换为字节切片，并赋值给 buf
	// Convert challengeKey to a byte slice and assign it to buf
	buf := []byte(challengeKey)

	// 将 MagicNumber 追加到 buf 的末尾
	// Append MagicNumber to the end of buf
	buf = append(buf, MagicNumber...)

	// 将 buf 写入到 h
	// Write buf to h
	h.Write(buf)

	// 计算 h 的 SHA-1 哈希值，并将其转换为 Base64 编码
	// Calculate the SHA-1 hash of h and convert it to Base64 encoding
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// NewMaskKey 是一个函数，它返回一个长度为 4 的字节构成的随机数切片
// NewMaskKey is a function that returns a random number slice consisting of 4 bytes
func NewMaskKey() [4]byte {
	// 调用 AlphabetNumeric.Uint32() 方法生成一个 uint32 类型的随机数 n
	// Call the AlphabetNumeric.Uint32() method to generate a random number n of type uint32
	n := AlphabetNumeric.Uint32()

	// 返回一个长度为 4 的字节切片，每个字节是 n(随机数) 的一个字节
	// Return a byte slice of length 4, each byte is a byte of n(random number)
	return [4]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}
}

// MethodExists 是一个函数，判断一个对象是否存在某个方法。
// MethodExists is a function that determines whether an object has a method.
func MethodExists(in any, method string) (reflect.Value, bool) {
	// 如果 in 为 nil 或 method 为空字符串，那么返回一个空的 reflect.Value 和 false。
	// If in is nil or method is an empty string, then return an empty reflect.Value and false.
	if in == nil || method == "" {
		return reflect.Value{}, false
	}

	// 获取 in 的类型，并赋值给 p。
	// Get the type of in and assign it to p.
	p := reflect.TypeOf(in)

	// 如果 p 的种类（Kind）是指针，那么获取 p 的元素类型。
	// If the kind of p is a pointer, then get the element type of p.
	if p.Kind() == reflect.Ptr {
		p = p.Elem()
	}

	// 如果 p 的种类（Kind）不是结构体，那么返回一个空的 reflect.Value 和 false。
	// If the kind of p is not a struct, then return an empty reflect.Value and false.
	if p.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	// 获取 in 的值，并赋值给 object。
	// Get the value of in and assign it to object.
	object := reflect.ValueOf(in)

	// 通过 method 名称获取 object 的方法，并赋值给 newMethod。
	// Get the method of object by the name of method and assign it to newMethod.
	newMethod := object.MethodByName(method)

	// 如果 newMethod 不是有效的，那么返回一个空的 reflect.Value 和 false。
	// If newMethod is not valid, then return an empty reflect.Value and false.
	if !newMethod.IsValid() {
		return reflect.Value{}, false
	}

	// 返回 newMethod 和 true。
	// Return newMethod and true.
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

// FnvString 函数接收一个字符串 s，然后使用 FNV-1a 哈希算法计算其哈希值。
// The FnvString function takes a string s and calculates its hash value using the FNV-1a hash algorithm.
func FnvString(s string) uint64 {
	// 初始化哈希值为 offset64
	// Initialize the hash value to offset64
	var h = uint64(offset64)

	// 遍历字符串 s 中的每个字符
	// Iterate over each character in the string s
	for _, b := range s {
		// 将哈希值乘以 prime64
		// Multiply the hash value by prime64
		h *= prime64

		// 将哈希值与字符的 ASCII 值进行异或操作
		// XOR the hash value with the ASCII value of the character
		h ^= uint64(b)
	}

	// 返回计算得到的哈希值
	// Return the calculated hash value
	return h
}

// FnvNumber 函数接收一个整数 x，然后使用 FNV-1a 哈希算法计算其哈希值。
// The FnvNumber function takes an integer x and calculates its hash value using the FNV-1a hash algorithm.
func FnvNumber[T Integer](x T) uint64 {
	// 初始化哈希值为 offset64
	// Initialize the hash value to offset64
	var h = uint64(offset64)

	// 将哈希值乘以 prime64
	// Multiply the hash value by prime64
	h *= prime64

	// 将哈希值与整数 x 进行异或操作
	// XOR the hash value with the integer x
	h ^= uint64(x)

	// 返回计算得到的哈希值
	// Return the calculated hash value
	return h
}

// MaskXOR 是一个函数，它接受两个字节切片作为参数：b 和 key。
// 它使用 key 对 b 进行异或操作，然后将结果存储在 b 中。
// MaskXOR is a function that takes two byte slices as arguments: b and key.
// It performs an XOR operation on b using key, and then stores the result in b.
func MaskXOR(b []byte, key []byte) {
	// 将 key 转换为小端序的 uint32，然后转换为 uint64，并将其复制到 key64 的高位和低位。
	// Convert key to a little-endian uint32, then convert it to a uint64, and copy it to the high and low bits of key64.
	var maskKey = binary.LittleEndian.Uint32(key)
	var key64 = uint64(maskKey)<<32 + uint64(maskKey)

	// 当 b 的长度大于或等于 64 时，将 b 的每 8 个字节与 key64 进行异或操作。
	// When the length of b is greater than or equal to 64, XOR every 8 bytes of b with key64.
	for len(b) >= 64 {
		// 读取 b 的前 8 个字节，并将其解释为小端序的 uint64
		// Read the first 8 bytes of b and interpret it as a little-endian uint64
		v := binary.LittleEndian.Uint64(b)

		// 将 v 与 key64 进行异或操作，然后将结果写回 b 的前 8 个字节
		// XOR v with key64 and then write the result back to the first 8 bytes of b
		binary.LittleEndian.PutUint64(b, v^key64)

		// 以下代码块重复上述操作，但是对 b 的不同部分进行操作
		// The following code blocks repeat the above operation, but operate on different parts of b
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

		// 将 b 的前 64 个字节移除，以便在下一次循环中处理剩余的字节
		// Remove the first 64 bytes of b so that the remaining bytes can be processed in the next loop
		b = b[64:]
	}

	// 当 b 的长度小于 64 但大于或等于 8 时，将 b 的每 8 个字节与 key64 进行异或操作。
	// When the length of b is less than 64 but greater than or equal to 8, XOR every 8 bytes of b with key64.
	for len(b) >= 8 {
		// 读取 b 的前 8 个字节，并将其解释为小端序的 uint64
		// Read the first 8 bytes of b and interpret it as a little-endian uint64
		v := binary.LittleEndian.Uint64(b[:8])

		// 将 v 与 key64 进行异或操作，然后将结果写回 b 的前 8 个字节
		// XOR v with key64 and then write the result back to the first 8 bytes of b
		binary.LittleEndian.PutUint64(b[:8], v^key64)

		// 将 b 的前 8 个字节移除，以便在下一次循环中处理剩余的字节
		// Remove the first 8 bytes of b so that the remaining bytes can be processed in the next loop
		b = b[8:]
	}

	// 当 b 的长度小于 8 时，将 b 的每个字节与 key 的相应字节进行异或操作。
	// When the length of b is less than 8, XOR each byte of b with the corresponding byte of key.
	var n = len(b)
	for i := 0; i < n; i++ {
		// 计算 key 的索引，这里使用了位运算符 &，它会返回两个数字的二进制表示中都为 1 的位的结果
		// Calculate the index of key, here we use the bitwise operator &, it will return the result of the bits that are 1 in the binary representation of both numbers
		idx := i & 3

		// 将 b 的第 i 个字节与 key 的第 idx 个字节进行异或操作，然后将结果写回 b 的第 i 个字节
		// XOR the i-th byte of b with the idx-th byte of key, and then write the result back to the i-th byte of b
		b[i] ^= key[idx]
	}
}

// InCollection 函数检查给定的字符串 elem 是否在字符串切片 elems 中。
// The InCollection function checks if the given string elem is in the string slice elems.
func InCollection(elem string, elems []string) bool {
	// 遍历 elems 中的每个元素
	// Iterate over each element in elems
	for _, item := range elems {
		// 如果找到了与 elem 相等的元素，返回 true
		// If an element equal to elem is found, return true
		if item == elem {
			return true
		}
	}

	// 如果没有找到与 elem 相等的元素，返回 false
	// If no element equal to elem is found, return false
	return false
}

// GetIntersectionElem 函数获取两个字符串切片 a 和 b 的交集中的一个元素。
// The GetIntersectionElem function gets an element in the intersection of two string slices a and b.
func GetIntersectionElem(a, b []string) string {
	// 遍历 a 中的每个元素
	// Iterate over each element in a
	for _, item := range a {
		// 如果 item 在 b 中，返回 item
		// If item is in b, return item
		if InCollection(item, b) {
			return item
		}
	}

	// 如果 a 和 b 没有交集，返回空字符串
	// If a and b have no intersection, return an empty string
	return ""
}

// Split 函数分割给定的字符串 s，使用 sep 作为分隔符。空值将会被过滤掉。
// The Split function splits the given string s using sep as the separator. Empty values will be filtered out.
func Split(s string, sep string) []string {
	// 使用 sep 分割 s，得到一个字符串切片
	// Split s using sep to get a string slice
	var list = strings.Split(s, sep)

	// 初始化一个索引 j
	// Initialize an index j
	var j = 0

	// 遍历 list 中的每个元素
	// Iterate over each element in list
	for _, v := range list {
		// 去除 v 的前后空白字符
		// Remove the leading and trailing white space of v
		if v = strings.TrimSpace(v); v != "" {
			// 如果 v 不为空，将其添加到 list 的 j 索引处，并将 j 加 1
			// If v is not empty, add it to the j index of list and increment j by 1
			list[j] = v
			j++
		}
	}

	// 返回 list 的前 j 个元素，即去除了空值的部分
	// Return the first j elements of list, i.e., the part without empty values
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

// BinaryPow 函数接收一个整数 n，然后计算并返回 2 的 n 次方。
// The BinaryPow function takes an integer n and then calculates and returns 2 to the power of n.
func BinaryPow(n int) int {
	// 初始化答案为 1
	// Initialize the answer to 1
	var ans = 1

	// 循环 n 次, 持续左移
	// Loop n times, continue to shift left
	for i := 0; i < n; i++ {
		// 将答案左移一位，这相当于将答案乘以 2
		// Shift the answer to the left by one bit, which is equivalent to multiplying the answer by 2
		ans <<= 1
	}

	// 返回计算得到的答案
	// Return the calculated answer
	return ans
}

// BufferReset 函数接收一个字节缓冲区 b 和一个字节切片 p，然后将 b 的底层切片重置为 p。
// The BufferReset function takes a byte buffer b and a byte slice p, and then resets the underlying slice of b to p.
// 注意：修改后面的属性一定要加偏移量，否则可能会导致未定义的行为。
// Note: Be sure to add an offset when modifying the following properties, otherwise it may lead to undefined behavior.
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
