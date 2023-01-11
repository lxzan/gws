package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"unsafe"
)

type webSocketMocker struct {
	onMessage func(socket *Conn, message *Message)
	onPing    func(socket *Conn, payload []byte)
	onPong    func(socket *Conn, payload []byte)
	onClose   func(socket *Conn, code uint16, reason []byte)
	onError   func(socket *Conn, err error)
}

func (c *webSocketMocker) OnOpen(socket *Conn) {
}

func (c *webSocketMocker) OnError(socket *Conn, err error) {
	if c.onError != nil {
		c.onError(socket, err)
	}
}

func (c *webSocketMocker) OnClose(socket *Conn, code uint16, reason []byte) {
	if c.onClose != nil {
		c.onClose(socket, code, reason)
	}
}

func (c *webSocketMocker) OnPing(socket *Conn, payload []byte) {
	if c.onPing != nil {
		c.onPing(socket, payload)
	}
}

func (c *webSocketMocker) OnPong(socket *Conn, payload []byte) {
	if c.onPong != nil {
		c.onPong(socket, payload)
	}
}

func (c *webSocketMocker) OnMessage(socket *Conn, message *Message) {
	if c.onMessage != nil {
		c.onMessage(socket, message)
	}
}

func (c *webSocketMocker) writeToReader(conn *Conn, reader *bytes.Buffer, row testRow, payload []byte) error {
	var copiedText = make([]byte, len(payload))
	copy(copiedText, payload)

	var opcode = Opcode(row.Opcode)
	var compressEnabled = conn.compressEnabled && opcode.IsDataFrame()
	compressedText, err := conn.compressor.Compress(copiedText)
	if err != nil {
		return err
	}

	var n = len(copiedText)
	if compressEnabled {
		n = len(compressedText)
	}

	var fh = frameHeader{}
	var key = internal.NewMaskKey()
	var offset = fh.GenerateServerHeader(row.Fin, compressEnabled, opcode, n)
	if row.RSV2 {
		fh[0] += 32
	}
	fh.SetMask()
	fh.SetMaskKey(offset, key)
	reader.Write(fh[:offset+4])

	if compressEnabled {
		maskXOR(compressedText, key[0:])
		reader.Write(compressedText)
	} else {
		maskXOR(copiedText, key[0:])
		reader.Write(copiedText)
	}

	return nil
}

func newHttpWriter() *httpWriter {
	server, client := net.Pipe()
	var r = bytes.NewBuffer(nil)
	var w = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	go func() {
		for {
			var p [1024]byte
			if _, err := client.Read(p[0:]); err != nil {
				return
			}
		}
	}()

	return &httpWriter{
		conn: server,
		brw:  brw,
	}
}

type httpWriter struct {
	conn net.Conn
	brw  *bufio.ReadWriter
}

func (c *httpWriter) Header() http.Header {
	return http.Header{}
}

func (c *httpWriter) Write(i []byte) (int, error) {
	return 0, nil
}

func (c *httpWriter) WriteHeader(statusCode int) {}

func (c *httpWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return c.conn, c.brw, nil
}

func BenchmarkNewBuffer(b *testing.B) {
	var str = internal.AlphabetNumeric.Generate(1024)
	var bs = *(*[]byte)(unsafe.Pointer(&str))
	for i := 0; i < b.N; i++ {
		bytes.NewBuffer(bs)
	}
}

func BenchmarkCompress(b *testing.B) {
	var s = internal.AlphabetNumeric.Generate(1024)
	var buf = bytes.NewBuffer(nil)
	fw, _ := flate.NewWriter(nil, -2)
	for i := 0; i < b.N; i++ {
		buf.Reset()
		fw.Reset(buf)
		fw.Write(s)
		fw.Flush()
		fw.Close()
	}
}

func BenchmarkDeCompress(b *testing.B) {
	var s = internal.AlphabetNumeric.Generate(1024)
	var buf = bytes.NewBuffer(nil)
	fw, _ := flate.NewWriter(buf, -2)
	fw.Write(s)
	fw.Flush()
	fw.Close()
	fr := flate.NewReader(nil)

	var result = bytes.NewBuffer(nil)
	result.Grow(1024)
	for i := 0; i < b.N; i++ {
		r := bytes.NewBuffer(buf.Bytes())
		fr.(flate.Resetter).Reset(r, nil)
		result.Reset()
		io.Copy(result, fr)
	}
}

func BenchmarkMask(b *testing.B) {
	var s1 = internal.AlphabetNumeric.Generate(1280)
	var s2 = s1
	var key [4]byte
	binary.LittleEndian.PutUint32(key[:4], internal.AlphabetNumeric.Uint32())
	for i := 0; i < b.N; i++ {
		maskXOR(s2, key[:4])
	}
}

func BenchmarkSyncMap_Store(b *testing.B) {
	var m = sync.Map{}
	for i := 0; i < b.N; i++ {
		var key = string(internal.AlphabetNumeric.Generate(16))
		m.Store(key, 1)
	}
}

func BenchmarkSyncMap_Load(b *testing.B) {
	var m = sync.Map{}
	for i := 0; i < 10000; i++ {
		var key = string(internal.AlphabetNumeric.Generate(4))
		m.Store(key, 1)
	}
	for i := 0; i < b.N; i++ {
		var key = string(internal.AlphabetNumeric.Generate(4))
		m.Load(key)
	}
}

func TestMask(t *testing.T) {
	for i := 0; i < 1000; i++ {
		var s1 = internal.AlphabetNumeric.Generate(1280)
		var s2 = make([]byte, len(s1))
		copy(s2, s1)

		var key = make([]byte, 4, 4)
		binary.LittleEndian.PutUint32(key, internal.AlphabetNumeric.Uint32())
		maskXOR(s1, key)
		internal.MaskByByte(s2, key)
		for i, _ := range s1 {
			if s1[i] != s2[i] {
				t.Fail()
			}
		}
	}
}
