package gws

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/klauspost/compress/flate"
	"github.com/lxzan/gws/internal"
)

// flateTail 是一个字节切片，用于表示 deflate 压缩算法的尾部标记。
// flateTail is a byte slice, used to represent the tail marker of the deflate compression algorithm.
var flateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

// deflaterPool 是一个结构体，用于表示一个 deflater 对象的池。
// deflaterPool is a struct, used to represent a pool of deflater objects.
type deflaterPool struct {
	// serial 是一个无符号64位整数，用于表示 deflaterPool 的序列号。
	// serial is an unsigned 64-bit integer, used to represent the serial number of the deflaterPool.
	serial uint64

	// num 是一个无符号64位整数，用于表示 deflaterPool 中 deflater 对象的数量。
	// num is an unsigned 64-bit integer, used to represent the number of deflater objects in the deflaterPool.
	num uint64

	// pool 是一个 deflater 对象的切片，用于存储 deflaterPool 中的 deflater 对象。
	// pool is a slice of deflater objects, used to store the deflater objects in the deflaterPool.
	pool []*deflater
}

// initialize 是 deflaterPool 结构体的一个方法，用于初始化 deflaterPool。
// initialize is a method of the deflaterPool struct, used to initialize the deflaterPool.
func (c *deflaterPool) initialize(options PermessageDeflate, limit int) *deflaterPool {
	// 设置 deflaterPool 的大小为 options 的 PoolSize 属性值
	// Set the size of the deflaterPool to the PoolSize property value of options
	c.num = uint64(options.PoolSize)

	// 循环创建 deflater 对象并添加到 deflaterPool 中
	// Loop to create deflater objects and add them to the deflaterPool
	for i := uint64(0); i < c.num; i++ {
		// 创建一个新的 deflater 对象并初始化，然后添加到 deflaterPool 的 pool 切片中
		// Create a new deflater object and initialize it, then add it to the pool slice of the deflaterPool
		c.pool = append(c.pool, new(deflater).initialize(true, options, limit))
	}

	// 返回初始化后的 deflaterPool 对象
	// Return the initialized deflaterPool object
	return c
}

// Select 是 deflaterPool 结构体的一个方法，用于从 deflaterPool 中选择一个 deflater 对象。
// Select is a method of the deflaterPool struct, used to select a deflater object from the deflaterPool.
func (c *deflaterPool) Select() *deflater {
	// 使用原子操作增加 deflaterPool 的序列号，并与 deflaterPool 的大小减一进行按位与运算，得到一个索引值
	// Use atomic operation to increase the serial number of the deflaterPool, and perform bitwise AND operation with the size of the deflaterPool minus one to get an index value
	var j = atomic.AddUint64(&c.serial, 1) & (c.num - 1)

	// 返回 deflaterPool 中索引为 j 的 deflater 对象
	// Return the deflater object at index j in the deflaterPool
	return c.pool[j]
}

// deflater 是一个结构体，用于表示一个 deflate 压缩器。
// deflater is a struct, used to represent a deflate compressor.
type deflater struct {
	// dpsLocker 是一个互斥锁，用于保护 dpsBuffer 和 dpsReader 的并发访问。
	// dpsLocker is a mutex, used to protect concurrent access to dpsBuffer and dpsReader.
	dpsLocker sync.Mutex

	// buf 是一个字节切片，用于存储压缩数据。
	// buf is a byte slice, used to store compressed data.
	buf []byte

	// limit 是一个整数，用于表示压缩数据的大小限制。
	// limit is an integer, used to represent the size limit of compressed data.
	limit int

	// dpsBuffer 是一个字节缓冲，用于存储待压缩的数据。
	// dpsBuffer is a byte buffer, used to store data to be compressed.
	dpsBuffer *bytes.Buffer

	// dpsReader 是一个读取器，用于从 dpsBuffer 中读取数据。
	// dpsReader is a reader, used to read data from dpsBuffer.
	dpsReader io.ReadCloser

	// cpsLocker 是一个互斥锁，用于保护 cpsWriter 的并发访问。
	// cpsLocker is a mutex, used to protect concurrent access to cpsWriter.
	cpsLocker sync.Mutex

	// cpsWriter 是一个写入器，用于向目标写入压缩后的数据。
	// cpsWriter is a writer, used to write compressed data to the target.
	cpsWriter *flate.Writer
}

// initialize 是 deflater 结构体的一个方法，用于初始化 deflater。
// initialize is a method of the deflater struct, used to initialize the deflater.
func (c *deflater) initialize(isServer bool, options PermessageDeflate, limit int) *deflater {
	// 创建一个新的 deflate 读取器
	// Create a new deflate reader
	c.dpsReader = flate.NewReader(nil)

	// 创建一个新的字节缓冲
	// Create a new byte buffer
	c.dpsBuffer = bytes.NewBuffer(nil)

	// 创建一个大小为 32*1024 的字节切片
	// Create a byte slice of size 32*1024
	c.buf = make([]byte, 32*1024)

	// 设置压缩数据的大小限制
	// Set the size limit of compressed data
	c.limit = limit

	// 根据是否是服务器，选择服务器或客户端的最大窗口位数
	// Select the maximum window bits of the server or client depending on whether it is a server
	windowBits := internal.SelectValue(isServer, options.ServerMaxWindowBits, options.ClientMaxWindowBits)

	// 如果窗口位数为 15
	// If the window bits is 15
	if windowBits == 15 {
		// 创建一个新的 deflate 写入器，压缩级别为 options.Level
		// Create a new deflate writer with a compression level of options.Level
		c.cpsWriter, _ = flate.NewWriter(nil, options.Level)
	} else {
		// 创建一个新的 deflate 写入器，窗口大小为 2 的 windowBits 次方
		// Create a new deflate writer with a window size of 2 to the power of windowBits
		c.cpsWriter, _ = flate.NewWriterWindow(nil, internal.BinaryPow(windowBits))
	}

	// 返回初始化后的 deflater 对象
	// Return the initialized deflater object
	return c
}

// resetFR 是 deflater 结构体的一个方法，用于重置 deflate 读取器和字节缓冲。
// resetFR is a method of the deflater struct, used to reset the deflate reader and byte buffer.
func (c *deflater) resetFR(r io.Reader, dict []byte) {
	// 获取 deflate 读取器的 Resetter 接口
	// Get the Resetter interface of the deflate reader
	resetter := c.dpsReader.(flate.Resetter)

	// 使用新的读取器和字典重置 deflate 读取器
	// Reset the deflate reader with a new reader and dictionary
	_ = resetter.Reset(r, dict) // must return a null pointer

	// 如果字节缓冲的容量大于 256*1024，则创建一个新的字节缓冲
	// If the capacity of the byte buffer is greater than 256*1024, create a new byte buffer
	if c.dpsBuffer.Cap() > 256*1024 {
		c.dpsBuffer = bytes.NewBuffer(nil)
	}

	// 重置字节缓冲
	// Reset the byte buffer
	c.dpsBuffer.Reset()
}

// Decompress 是 deflater 结构体的一个方法，用于解压缩数据。
// Decompress is a method of the deflater struct, used to decompress data.
func (c *deflater) Decompress(src *bytes.Buffer, dict []byte) (*bytes.Buffer, error) {
	// 加锁，保护 dpsBuffer 和 dpsReader 的并发访问
	// Lock to protect concurrent access to dpsBuffer and dpsReader
	c.dpsLocker.Lock()

	// 函数返回时解锁
	// Unlock when the function returns
	defer c.dpsLocker.Unlock()

	// 将 deflate 压缩算法的尾部标记写入源数据
	// Write the tail marker of the deflate compression algorithm into the source data
	_, _ = src.Write(flateTail)

	// 重置 deflate 读取器和字节缓冲
	// Reset the deflate reader and byte buffer
	c.resetFR(src, dict)

	// 创建一个限制读取器，限制读取的数据大小不超过 c.limit
	// Create a limit reader, limiting the size of the data read to not exceed c.limit
	reader := limitReader(c.dpsReader, c.limit)

	// 将 reader 中的数据复制到 dpsBuffer 中，使用 c.buf 作为缓冲
	// Copy the data in reader to dpsBuffer, using c.buf as a buffer
	if _, err := io.CopyBuffer(c.dpsBuffer, reader, c.buf); err != nil {
		// 如果复制过程中出现错误，返回 nil 和错误信息
		// If an error occurs during the copy, return nil and the error message
		return nil, err
	}

	// 从二进制池中获取一个新的字节缓冲，大小为 dpsBuffer 的长度
	// Get a new byte buffer from the binary pool, the size is the length of dpsBuffer
	var dst = binaryPool.Get(c.dpsBuffer.Len())

	// 将 dpsBuffer 中的数据写入 dst
	// Write the data in dpsBuffer to dst
	_, _ = c.dpsBuffer.WriteTo(dst)

	// 返回 dst 和 nil
	// Return dst and nil
	return dst, nil
}

// Compress 是 deflater 结构体的一个方法，用于压缩数据。
// Compress is a method of the deflater struct, used to compress data.
func (c *deflater) Compress(src internal.Payload, dst *bytes.Buffer, dict []byte) error {
	// 加锁，保护 cpsWriter 的并发访问
	// Lock to protect concurrent access to cpsWriter
	c.cpsLocker.Lock()

	// 函数返回时解锁
	// Unlock when the function returns
	defer c.cpsLocker.Unlock()

	// 使用新的字节缓冲和字典重置 cpsWriter
	// Reset cpsWriter with a new byte buffer and dictionary
	c.cpsWriter.ResetDict(dst, dict)

	// 将源数据写入 cpsWriter
	// Write the source data to cpsWriter
	if _, err := src.WriteTo(c.cpsWriter); err != nil {
		// 如果写入过程中出现错误，返回错误信息
		// If an error occurs during the write, return the error message
		return err
	}

	// 刷新 cpsWriter，将所有未写入的数据写入字节缓冲
	// Flush cpsWriter, write all unwritten data to the byte buffer
	if err := c.cpsWriter.Flush(); err != nil {
		// 如果刷新过程中出现错误，返回错误信息
		// If an error occurs during the flush, return the error message
		return err
	}

	// 如果字节缓冲的长度大于等于 4
	// If the length of the byte buffer is greater than or equal to 4
	if n := dst.Len(); n >= 4 {
		// 获取字节缓冲的字节切片
		// Get the byte slice of the byte buffer
		compressedContent := dst.Bytes()

		// 如果字节切片的尾部 4 个字节表示的无符号整数等于最大的 16 位无符号整数
		// If the unsigned integer represented by the last 4 bytes of the byte slice is equal to the maximum 16-bit unsigned integer
		if tail := compressedContent[n-4:]; binary.BigEndian.Uint32(tail) == math.MaxUint16 {
			// 截断字节缓冲，去掉尾部的 4 个字节
			// Truncate the byte buffer, remove the last 4 bytes
			dst.Truncate(n - 4)
		}
	}

	// 返回 nil，表示压缩成功
	// Return nil, indicating that the compression was successful
	return nil
}

// slideWindow 是一个结构体，用于表示一个滑动窗口。
// slideWindow is a struct, used to represent a sliding window.
type slideWindow struct {
	// enabled 是一个布尔值，表示滑动窗口是否启用。
	// enabled is a boolean value, indicating whether the sliding window is enabled.
	enabled bool

	// dict 是一个字节切片，用于存储滑动窗口的数据。
	// dict is a byte slice, used to store the data of the sliding window.
	dict []byte

	// size 是一个整数，表示滑动窗口的大小。
	// size is an integer, representing the size of the sliding window.
	size int
}

// initialize 是 slideWindow 结构体的一个方法，用于初始化滑动窗口。
// initialize is a method of the slideWindow struct, used to initialize the sliding window.
func (c *slideWindow) initialize(pool *internal.Pool[[]byte], windowBits int) *slideWindow {
	// 启用滑动窗口
	// Enable the sliding window
	c.enabled = true

	// 设置滑动窗口的大小为 2 的 windowBits 次方
	// Set the size of the sliding window to 2 to the power of windowBits
	c.size = internal.BinaryPow(windowBits)

	if pool != nil {
		// 如果池不为空，从池中获取一个字节切片，并设置其长度为 0
		// If the pool is not empty, get a byte slice from the pool and set its length to 0
		c.dict = pool.Get()[:0]
	} else {
		// 如果池为空，创建一个新的字节切片，长度为 0，容量为滑动窗口的大小
		// If the pool is empty, create a new byte slice with a length of 0 and a capacity of the size of the sliding window
		c.dict = make([]byte, 0, c.size)
	}

	// 返回初始化后的滑动窗口对象
	// Return the initialized sliding window object
	return c
}

// Write 是 slideWindow 结构体的一个方法，用于将数据写入滑动窗口。
// Write is a method of the slideWindow struct, used to write data to the sliding window.
func (c *slideWindow) Write(p []byte) (int, error) {
	// 如果滑动窗口未启用，返回 0 和 nil
	// If the sliding window is not enabled, return 0 and nil
	if !c.enabled {
		return 0, nil
	}

	// 获取 p 的长度
	// Get the length of p
	var total = len(p)

	// n 是待写入的数据长度
	// n is the length of the data to be written
	var n = total

	// 获取滑动窗口的长度
	// Get the length of the sliding window
	var length = len(c.dict)

	// 如果待写入的数据长度加上滑动窗口的长度小于等于滑动窗口的大小
	// If the length of the data to be written plus the length of the sliding window is less than or equal to the size of the sliding window
	if n+length <= c.size {
		// 将 p 添加到滑动窗口的末尾
		// Add p to the end of the sliding window
		c.dict = append(c.dict, p...)

		// 返回 p 的长度和 nil
		// Return the length of p and nil
		return total, nil
	}

	// 如果滑动窗口的大小减去滑动窗口的长度大于 0
	// If the size of the sliding window minus the length of the sliding window is greater than 0
	if m := c.size - length; m > 0 {
		// 将 p 的前 m 个元素添加到滑动窗口的末尾
		// Add the first m elements of p to the end of the sliding window
		c.dict = append(c.dict, p[:m]...)

		// 将 p 的前 m 个元素删除
		// Delete the first m elements of p
		p = p[m:]

		// 更新待写入的数据长度
		// Update the length of the data to be written
		n = len(p)
	}

	// 如果待写入的数据长度大于等于滑动窗口的大小
	// If the length of the data to be written is greater than or equal to the size of the sliding window
	if n >= c.size {
		// 将 p 的后 c.size 个元素复制到滑动窗口
		// Copy the last c.size elements of p to the sliding window
		copy(c.dict, p[n-c.size:])

		// 返回 p 的长度和 nil
		// Return the length of p and nil
		return total, nil
	}

	// 将滑动窗口的后 n 个元素复制到滑动窗口的前面
	// Copy the last n elements of the sliding window to the front of the sliding window
	copy(c.dict, c.dict[n:])

	// 将 p 复制到滑动窗口的后面
	// Copy p to the back of the sliding window
	copy(c.dict[c.size-n:], p)

	// 返回 p 的长度和 nil
	// Return the length of p and nil
	return total, nil
}

// genRequestHeader 是 PermessageDeflate 结构体的一个方法，用于生成请求头。
// genRequestHeader is a method of the PermessageDeflate struct, used to generate request headers.
func (c *PermessageDeflate) genRequestHeader() string {
	// 创建一个字符串切片，长度为 0，容量为 5
	// Create a string slice with a length of 0 and a capacity of 5
	var options = make([]string, 0, 5)

	// 将 PermessageDeflate 添加到 options
	// Add PermessageDeflate to options
	options = append(options, internal.PermessageDeflate)

	// 如果 ServerContextTakeover 为 false
	// If ServerContextTakeover is false
	if !c.ServerContextTakeover {
		// 将 ServerNoContextTakeover 添加到 options
		// Add ServerNoContextTakeover to options
		options = append(options, internal.ServerNoContextTakeover)
	}

	// 如果 ClientContextTakeover 为 false
	// If ClientContextTakeover is false
	if !c.ClientContextTakeover {
		// 将 ClientNoContextTakeover 添加到 options
		// Add ClientNoContextTakeover to options
		options = append(options, internal.ClientNoContextTakeover)
	}

	// 如果 ServerMaxWindowBits 不等于 15
	// If ServerMaxWindowBits is not equal to 15
	if c.ServerMaxWindowBits != 15 {
		// 将 ServerMaxWindowBits 和其值添加到 options
		// Add ServerMaxWindowBits and its value to options
		options = append(options, internal.ServerMaxWindowBits+internal.EQ+strconv.Itoa(c.ServerMaxWindowBits))
	}

	// 如果 ClientMaxWindowBits 不等于 15
	// If ClientMaxWindowBits is not equal to 15
	if c.ClientMaxWindowBits != 15 {
		// 将 ClientMaxWindowBits 和其值添加到 options
		// Add ClientMaxWindowBits and its value to options
		options = append(options, internal.ClientMaxWindowBits+internal.EQ+strconv.Itoa(c.ClientMaxWindowBits))
	} else if c.ClientContextTakeover {
		// 如果 ClientContextTakeover 为 true
		// If ClientContextTakeover is true
		// 将 ClientMaxWindowBits 添加到 options
		// Add ClientMaxWindowBits to options
		options = append(options, internal.ClientMaxWindowBits)
	}

	// 使用 "; " 将 options 中的所有元素连接成一个字符串，并返回
	// Join all elements in options into a string using "; " and return
	return strings.Join(options, "; ")
}

// genResponseHeader 是 PermessageDeflate 结构体的一个方法，用于生成响应头。
// genResponseHeader is a method of the PermessageDeflate struct, used to generate response headers.
func (c *PermessageDeflate) genResponseHeader() string {
	// 创建一个字符串切片，长度为 0，容量为 5
	// Create a string slice with a length of 0 and a capacity of 5
	var options = make([]string, 0, 5)

	// 将 PermessageDeflate 添加到 options
	// Add PermessageDeflate to options
	options = append(options, internal.PermessageDeflate)

	// 如果 ServerContextTakeover 为 false
	// If ServerContextTakeover is false
	if !c.ServerContextTakeover {
		// 将 ServerNoContextTakeover 添加到 options
		// Add ServerNoContextTakeover to options
		options = append(options, internal.ServerNoContextTakeover)
	}

	// 如果 ClientContextTakeover 为 false
	// If ClientContextTakeover is false
	if !c.ClientContextTakeover {
		// 将 ClientNoContextTakeover 添加到 options
		// Add ClientNoContextTakeover to options
		options = append(options, internal.ClientNoContextTakeover)
	}

	// 如果 ServerMaxWindowBits 不等于 15
	// If ServerMaxWindowBits is not equal to 15
	if c.ServerMaxWindowBits != 15 {
		// 将 ServerMaxWindowBits 和其值添加到 options
		// Add ServerMaxWindowBits and its value to options
		options = append(options, internal.ServerMaxWindowBits+internal.EQ+strconv.Itoa(c.ServerMaxWindowBits))
	}

	// 如果 ClientMaxWindowBits 不等于 15
	// If ClientMaxWindowBits is not equal to 15
	if c.ClientMaxWindowBits != 15 {
		// 将 ClientMaxWindowBits 和其值添加到 options
		// Add ClientMaxWindowBits and its value to options
		options = append(options, internal.ClientMaxWindowBits+internal.EQ+strconv.Itoa(c.ClientMaxWindowBits))
	}

	// 使用 "; " 将 options 中的所有元素连接成一个字符串，并返回
	// Join all elements in options into a string using "; " and return
	return strings.Join(options, "; ")
}

// permessageNegotiation 是一个函数，用于解析 permessage-deflate 扩展头。
// permessageNegotiation is a function used to parse the permessage-deflate extension header.
func permessageNegotiation(str string) PermessageDeflate {
	// 创建一个 PermessageDeflate 结构体 options，并初始化其属性。
	// Create a PermessageDeflate struct options and initialize its properties.
	var options = PermessageDeflate{

		// ServerContextTakeover 属性设置为 true，表示服务器可以接管上下文。
		// The ServerContextTakeover property is set to true, indicating that the server can take over the context.
		ServerContextTakeover: true,

		// ClientContextTakeover 属性设置为 true，表示客户端可以接管上下文。
		// The ClientContextTakeover property is set to true, indicating that the client can take over the context.
		ClientContextTakeover: true,

		// ServerMaxWindowBits 属性设置为 15，表示服务器的最大窗口位数为 15。
		// The ServerMaxWindowBits property is set to 15, indicating that the maximum window bits for the server is 15.
		ServerMaxWindowBits: 15,

		// ClientMaxWindowBits 属性设置为 15，表示客户端的最大窗口位数为 15。
		// The ClientMaxWindowBits property is set to 15, indicating that the maximum window bits for the client is 15.
		ClientMaxWindowBits: 15,
	}

	// 将 str 以 ";" 为分隔符进行分割，得到一个字符串切片 ss
	// Split the string str by ";" to get a string slice ss
	var ss = internal.Split(str, ";")

	// 遍历 ss 中的每一个字符串 s
	// Iterate over each string s in ss
	for _, s := range ss {

		// 将 s 以 "=" 为分隔符进行分割，得到一个字符串切片 pair
		// Split the string s by "=" to get a string slice pair
		var pair = strings.SplitN(s, "=", 2)

		// 根据 pair[0] 的值进行判断
		// Judge based on the value of pair[0]
		switch pair[0] {

		// 如果 pair[0] 的值为 PermessageDeflate 或者 ServerNoContextTakeover，则将 options 的 ServerContextTakeover 属性设置为 false
		// If the value of pair[0] is PermessageDeflate or ServerNoContextTakeover, set the ServerContextTakeover property of options to false
		case internal.PermessageDeflate:
		case internal.ServerNoContextTakeover:
			options.ServerContextTakeover = false

		// 如果 pair[0] 的值为 ClientNoContextTakeover，则将 options 的 ClientContextTakeover 属性设置为 false
		// If the value of pair[0] is ClientNoContextTakeover, set the ClientContextTakeover property of options to false
		case internal.ClientNoContextTakeover:
			options.ClientContextTakeover = false

		// 如果 pair[0] 的值为 ServerMaxWindowBits
		// If the value of pair[0] is ServerMaxWindowBits
		case internal.ServerMaxWindowBits:
			// 如果 pair 的长度为 2
			// If the length of pair is 2
			if len(pair) == 2 {
				// 将 pair[1] 转换为整数 x
				// Convert pair[1] to integer x
				x, _ := strconv.Atoi(pair[1])

				// 如果 x 为 0，则将 x 设置为 15
				// If x is 0, set x to 15
				x = internal.WithDefault(x, 15)

				// 将 options 的 ServerMaxWindowBits 属性设置为 options 的 ServerMaxWindowBits 属性和 x 中的较小值
				// Set the ServerMaxWindowBits property of options to the smaller of the ServerMaxWindowBits property of options and x
				options.ServerMaxWindowBits = internal.Min(options.ServerMaxWindowBits, x)
			}

		// 如果 pair[0] 的值为 ClientMaxWindowBits
		// If the value of pair[0] is ClientMaxWindowBits
		case internal.ClientMaxWindowBits:
			// 如果 pair 的长度为 2
			// If the length of pair is 2
			if len(pair) == 2 {
				// 将 pair[1] 转换为整数 x
				// Convert pair[1] to integer x
				x, _ := strconv.Atoi(pair[1])

				// 如果 x 为 0，则将 x 设置为 15
				// If x is 0, set x to 15
				x = internal.WithDefault(x, 15)

				// 将 options 的 ClientMaxWindowBits 属性设置为 options 的 ClientMaxWindowBits 属性和 x 中的较小值
				// Set the ClientMaxWindowBits property of options to the smaller of the ClientMaxWindowBits property of options and x
				options.ClientMaxWindowBits = internal.Min(options.ClientMaxWindowBits, x)
			}
		}
	}

	// 如果 options.ClientMaxWindowBits 小于 8，那么将 options.ClientMaxWindowBits 设置为 8，否则保持不变。
	// If options.ClientMaxWindowBits is less than 8, then set options.ClientMaxWindowBits to 8, otherwise keep it unchanged.
	options.ClientMaxWindowBits = internal.SelectValue(options.ClientMaxWindowBits < 8, 8, options.ClientMaxWindowBits)

	// 如果 options.ServerMaxWindowBits 小于 8，那么将 options.ServerMaxWindowBits 设置为 8，否则保持不变。
	// If options.ServerMaxWindowBits is less than 8, then set options.ServerMaxWindowBits to 8, otherwise keep it unchanged.
	options.ServerMaxWindowBits = internal.SelectValue(options.ServerMaxWindowBits < 8, 8, options.ServerMaxWindowBits)

	// 返回 options 结构体
	// Return the options struct
	return options
}

// limitReader 是一个函数，接收一个 io.Reader 和一个限制值，返回一个新的 limitedReader
// limitReader is a function that takes an io.Reader and a limit, and returns a new limitedReader
func limitReader(r io.Reader, limit int) io.Reader { return &limitedReader{R: r, M: limit} }

// limitedReader 是一个结构体，包含一个 io.Reader 和两个整数 N 和 M。
// limitedReader is a struct that contains an io.Reader and two integers N and M.
type limitedReader struct {
	// R 是一个 io.Reader，它是一个接口，用于读取数据。
	// R is an io.Reader, which is an interface used for reading data.
	R io.Reader

	// N 是一个整数，用于表示限制读取的字节数。
	// N is an integer, used to represent the number of bytes to limit the read.
	N int

	// M 是一个整数，用于表示读取的最大字节数。
	// M is an integer, used to represent the maximum number of bytes to read.
	M int
}

// Read 是 limitedReader 的一个方法，用于读取数据
// Read is a method of limitedReader, used to read data
func (c *limitedReader) Read(p []byte) (n int, err error) {
	// 从 c.R 中读取数据到 p，返回读取的数据量 n 和可能的错误 err
	// Read data from c.R into p, return the amount of data read n and possible error err
	n, err = c.R.Read(p)

	// 将读取的数据量加到 c.N
	// Add the amount of data read to c.N
	c.N += n

	// 如果已读取的数据量超过限制
	// If the amount of data read exceeds the limit
	if c.N > c.M {
		// 返回读取的数据量和一个错误信息
		// Return the amount of data read and an error message
		return n, internal.CloseMessageTooLarge
	}

	// 返回读取的数据量和可能的错误
	// Return the amount of data read and possible error
	return
}
