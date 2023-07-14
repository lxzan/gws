package gws

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

type webSocketMocker struct {
	sync.Mutex
	onMessage func(socket *Conn, message *Message)
	onPing    func(socket *Conn, payload []byte)
	onPong    func(socket *Conn, payload []byte)
	onClose   func(socket *Conn, err error)
}

func (c *webSocketMocker) reset(socket *Conn, reader *bytes.Buffer, writer *bytes.Buffer) {
	reader.Reset()
	writer.Reset()
	socket.rbuf.Reset(reader)
	atomic.StoreUint32(&socket.closed, 0)
}

func (c *webSocketMocker) OnOpen(socket *Conn) {
}

func (c *webSocketMocker) OnClose(socket *Conn, err error) {
	if c.onClose != nil {
		c.onClose(socket, err)
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

func TestOthers(t *testing.T) {
	conn, _ := net.Pipe()
	upgrader := NewUpgrader(new(BuiltinEventHandler), nil)
	socket := &Conn{
		conn:    conn,
		handler: new(webSocketMocker),
		config:  upgrader.option.getConfig(),
	}
	socket.SetDeadline(time.Time{})
	socket.SetReadDeadline(time.Time{})
	socket.SetWriteDeadline(time.Time{})
	socket.LocalAddr()
	socket.NetConn()
	socket.RemoteAddr()

	var as = assert.New(t)
	var fh = frameHeader{}
	fh.SetMask()
	var maskKey [4]byte
	copy(maskKey[:4], internal.AlphabetNumeric.Generate(4))
	fh.SetMaskKey(10, maskKey)
	as.Equal(true, fh.GetMask())
	as.Equal(string(maskKey[:4]), string(fh.GetMaskKey()))
	return
}

func TestConn_Close(t *testing.T) {
	conn, _ := net.Pipe()
	var socket = &Conn{conn: conn, closed: 1}
	assert.NoError(t, socket.SetDeadline(time.Time{}))
	assert.NoError(t, socket.SetReadDeadline(time.Time{}))
	assert.NoError(t, socket.SetWriteDeadline(time.Time{}))
}
