package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"github.com/lxzan/gws/internal"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type webSocketMocker struct {
	sync.Mutex
	onMessage func(socket *Conn, message *Message)
	onPing    func(socket *Conn, payload []byte)
	onPong    func(socket *Conn, payload []byte)
	onClose   func(socket *Conn, code uint16, reason []byte)
	onError   func(socket *Conn, err error)
}

func (c *webSocketMocker) reset(socket *Conn, reader *bytes.Buffer, writer *bytes.Buffer) {
	reader.Reset()
	writer.Reset()
	socket.rbuf.Reset(reader)
	socket.wbuf.Reset(writer)
	atomic.StoreUint32(&socket.closed, 0)
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
	compressedText, err := conn.compressor.Compress(bytes.NewBuffer(copiedText))
	if err != nil {
		return err
	}

	var n = len(copiedText)
	if compressEnabled {
		n = compressedText.Len()
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
		internal.MaskXOR(compressedText.Bytes(), key[0:])
		reader.Write(compressedText.Bytes())
	} else {
		internal.MaskXOR(copiedText, key[0:])
		reader.Write(copiedText)
	}

	return nil
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
		internal.MaskXOR(s2, key[:4])
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

func TestConn(t *testing.T) {
	conn, _ := net.Pipe()
	socket := &Conn{
		conn:    conn,
		handler: new(webSocketMocker),
		wmu:     &sync.Mutex{},
		wbuf:    bufio.NewWriter(bytes.NewBuffer(nil)),
		config:  NewUpgrader(),
	}
	socket.SetDeadline(time.Time{})
	socket.SetReadDeadline(time.Time{})
	socket.SetWriteDeadline(time.Time{})
	socket.LocalAddr()
	socket.NetConn()
	socket.RemoteAddr()
	socket.Close(1000, []byte("goodbye"))
	socket.Listen()
	new(internal.Buffer).ReadFrom()
	return
}
