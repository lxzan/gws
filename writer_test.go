package gws

import (
	"bufio"
	"bytes"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestConn_WriteMessage(t *testing.T) {
	var as = assert.New(t)
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(handler, nil)
	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	conn, _ := net.Pipe()
	var socket = serveWebSocket(upgrader.option.getConfig(), new(sliceMap), conn, brw, handler, false)

	t.Run("text v1", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		socket.WriteString("hello")
		var p = make([]byte, 7)
		_, _ = writer.Read(p)
		as.Equal("hello", string(p[2:]))
		var fh = frameHeader{}
		copy(fh[0:], p[:2])
		as.Equal(OpcodeText, fh.GetOpcode())
		as.Equal(true, fh.GetFIN())
		as.Equal(false, fh.GetRSV1())
		as.Equal(false, fh.GetRSV2())
		as.Equal(false, fh.GetRSV3())
		as.Equal(false, fh.GetMask())
		as.Equal(uint8(5), fh.GetLengthCode())
	})

	t.Run("binary v2", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var contentLength = 500
		var text = internal.AlphabetNumeric.Generate(contentLength)
		socket.WriteMessage(OpcodeBinary, text)
		var p = make([]byte, contentLength+4)
		_, _ = writer.Read(p)
		as.Equal(string(text), string(p[4:]))
		var fh = frameHeader{}
		copy(fh[0:], p[:2])
		as.Equal(OpcodeBinary, fh.GetOpcode())
		as.Equal(true, fh.GetFIN())
		as.Equal(false, fh.GetRSV1())
		as.Equal(false, fh.GetMask())
		as.Equal(uint8(126), fh.GetLengthCode())
	})

	t.Run("ping", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		socket.WritePing([]byte("ping"))
		var p = make([]byte, 6)
		_, _ = writer.Read(p)
		as.Equal("ping", string(p[2:]))
		var fh = frameHeader{}
		copy(fh[0:], p[:2])
		as.Equal(OpcodePing, fh.GetOpcode())
		as.Equal(true, fh.GetFIN())
		as.Equal(false, fh.GetRSV1())
		as.Equal(false, fh.GetMask())
		as.Equal(uint8(4), fh.GetLengthCode())
	})

	t.Run("pong", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		socket.WritePong(nil)
		var p = make([]byte, 6)
		_, _ = writer.Read(p)
		var fh = frameHeader{}
		copy(fh[0:], p[:2])
		as.Equal(OpcodePong, fh.GetOpcode())
		as.Equal(true, fh.GetFIN())
		as.Equal(false, fh.GetRSV1())
		as.Equal(false, fh.GetMask())
		as.Equal(uint8(0), fh.GetLengthCode())
	})

	t.Run("close", func(t *testing.T) {
		socket.closed = 1
		writer.Reset()
		socket.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(500))
		as.Equal(0, writer.Len())
	})
}

func TestConn_WriteMessageCompress(t *testing.T) {
	var as = assert.New(t)
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(handler, &ServerOption{
		CheckUtf8Enabled: true,
	})
	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	conn, _ := net.Pipe()
	var socket = serveWebSocket(upgrader.option.getConfig(), new(sliceMap), conn, brw, handler, true)

	// 消息长度低于阈值, 不压缩内容
	t.Run("text v1", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var n = 64
		var text = internal.AlphabetNumeric.Generate(n)
		socket.WriteMessage(OpcodeText, text)
		var compressedLength = writer.Len() - 2

		var p = make([]byte, 2)
		_, _ = writer.Read(p)
		as.Equal(string(text), writer.String())
		var fh = frameHeader{}
		copy(fh[0:], p[:2])
		as.Equal(OpcodeText, fh.GetOpcode())
		as.Equal(true, fh.GetFIN())
		as.Equal(false, fh.GetRSV1())
		as.Equal(false, fh.GetRSV2())
		as.Equal(false, fh.GetRSV3())
		as.Equal(false, fh.GetMask())
		as.Equal(uint8(compressedLength), fh.GetLengthCode())
	})

	t.Run("text v2", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var n = 1024
		var text = internal.AlphabetNumeric.Generate(n)
		socket.WriteMessage(OpcodeText, text)

		buffer, err := socket.decompressor.Decompress(bytes.NewBuffer(writer.Bytes()[4:]))
		if err != nil {
			as.NoError(err)
			return
		}

		as.Equal(string(text), string(buffer.Bytes()))
		var p = make([]byte, 4)
		_, _ = writer.Read(p)
		var fh = frameHeader{}
		copy(fh[0:], p)
		as.Equal(OpcodeText, fh.GetOpcode())
		as.Equal(true, fh.GetFIN())
		as.Equal(true, fh.GetRSV1())
		as.Equal(false, fh.GetRSV2())
		as.Equal(false, fh.GetRSV3())
		as.Equal(false, fh.GetMask())
		as.Equal(uint8(126), fh.GetLengthCode())
	})

	t.Run("write to closed socket", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var n = 1024
		var text = internal.AlphabetNumeric.Generate(n)
		socket.closed = 1
		as.Equal(internal.ErrConnClosed, socket.WriteMessage(OpcodeText, text))
	})
}

func TestWriteBigMessage(t *testing.T) {
	var upgrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		WriteMaxPayloadSize: 10,
	})
	srv, _ := testNewPeer(upgrader)
	var err = srv.WriteMessage(OpcodeText, internal.AlphabetNumeric.Generate(128))
	assert.Error(t, err)
}

func TestWriteClose(t *testing.T) {
	var upgrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		WriteMaxPayloadSize: 10,
	})
	srv, _ := testNewPeer(upgrader)
	srv.WriteClose(0, []byte("goodbye"))
}

func TestConn_WriteAsyncError(t *testing.T) {
	var as = assert.New(t)

	//t.Run("", func(t *testing.T) {
	//	server, _ := testNewPeer(NewUpgrader(func(c *Upgrader) {
	//		c.EventHandler = new(BuiltinEventHandler)
	//		c.AsyncWriteCap = 1
	//	}))
	//
	//	_ = server.writeMQ.Push(OpcodeText, nil)
	//	err := server.WriteAsync(OpcodeText, nil)
	//	as.Equal(internal.ErrWriteMessageQueueCapFull, err)
	//})

	t.Run("", func(t *testing.T) {
		var upgrader = NewUpgrader(new(BuiltinEventHandler), nil)
		server, _ := testNewPeer(upgrader)
		server.closed = 1
		err := server.WriteAsync(OpcodeText, nil)
		as.Equal(internal.ErrConnClosed, err)
	})
}
