package gws

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"math"
	"net"
	"testing"
)

func TestConn_ReadMessage(t *testing.T) {
	var as = assert.New(t)
	var handler = new(webSocketMocker)
	var config = &Config{CheckTextEncoding: true}
	config.initialize()
	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	var socket = serveWebSocket(config, &Request{}, &net.TCPConn{}, brw, handler, false)

	t.Run("ping", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var key = internal.NewMaskKey()
		var fh = frameHeader{}
		var n = internal.AlphabetNumeric.Intn(internal.ThresholdV1 + 1)
		var text = internal.AlphabetNumeric.Generate(n)
		fh.GenerateServerHeader(true, false, OpcodePing, n)
		fh.SetMask()
		fh.SetMaskKey(2, key)
		reader.Write(fh[:6])
		var temp = make([]byte, n)
		copy(temp, text)
		maskXOR(temp, key[0:])
		reader.Write(temp)

		handler.onPing = func(socket *Conn, payload []byte) {
			as.Equal(string(text), string(payload))
		}

		if err := socket.readMessage(); err != nil {
			t.Fail()
			return
		}
	})

	t.Run("pong", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var key = internal.NewMaskKey()
		var fh = frameHeader{}
		var n = internal.AlphabetNumeric.Intn(internal.ThresholdV1 + 1)
		var text = internal.AlphabetNumeric.Generate(n)
		fh.GenerateServerHeader(true, false, OpcodePong, n)
		fh.SetMask()
		fh.SetMaskKey(2, key)
		reader.Write(fh[:6])
		var temp = make([]byte, n)
		copy(temp, text)
		maskXOR(temp, key[0:])
		reader.Write(temp)

		handler.onPong = func(socket *Conn, payload []byte) {
			as.Equal(string(text), string(payload))
		}

		if err := socket.readMessage(); err != nil {
			t.Fail()
			return
		}
	})

	t.Run("message-v1", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var key = internal.NewMaskKey()
		var fh = frameHeader{}
		var n = internal.AlphabetNumeric.Intn(internal.ThresholdV1 + 1)
		var text = internal.AlphabetNumeric.Generate(n)
		fh.GenerateServerHeader(true, false, OpcodeText, n)
		fh.SetMask()
		fh.SetMaskKey(2, key)
		reader.Write(fh[:6])
		var temp = make([]byte, n)
		copy(temp, text)
		maskXOR(temp, key[0:])
		reader.Write(temp)

		handler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(OpcodeText, message.Typ())
			as.Equal(string(text), string(message.Bytes()))
		}

		if err := socket.readMessage(); err != nil {
			t.Fail()
			return
		}
	})

	t.Run("message-v3", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var key = internal.NewMaskKey()
		var fh = frameHeader{}
		var n = math.MaxUint16 * 2
		var text = internal.AlphabetNumeric.Generate(n)
		fh.GenerateServerHeader(true, false, OpcodeText, n)
		fh.SetMask()
		fh.SetMaskKey(10, key)
		reader.Write(fh[0:])
		var temp = make([]byte, n)
		copy(temp, text)
		maskXOR(temp, key[0:])
		reader.Write(temp)

		handler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(OpcodeText, message.Typ())
			as.Equal(string(text), string(message.Bytes()))
		}

		if err := socket.readMessage(); err != nil {
			t.Fail()
			return
		}
	})

	t.Run("segments", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var expectedText = ""
		{
			var key = internal.NewMaskKey()
			var fh = frameHeader{}
			var n = internal.AlphabetNumeric.Intn(internal.ThresholdV1 + 1)
			var text = internal.AlphabetNumeric.Generate(n)
			expectedText += string(text)
			fh.GenerateServerHeader(false, false, OpcodeText, n)
			fh.SetMask()
			fh.SetMaskKey(2, key)
			reader.Write(fh[:6])
			var temp = make([]byte, n)
			copy(temp, text)
			maskXOR(temp, key[0:])
			reader.Write(temp)
		}
		{
			var key = internal.NewMaskKey()
			var fh = frameHeader{}
			var n = internal.AlphabetNumeric.Intn(internal.ThresholdV1 + 1)
			var text = internal.AlphabetNumeric.Generate(n)
			expectedText += string(text)
			fh.GenerateServerHeader(true, false, OpcodeContinuation, n)
			fh.SetMask()
			fh.SetMaskKey(2, key)
			reader.Write(fh[:6])
			var temp = make([]byte, n)
			copy(temp, text)
			maskXOR(temp, key[0:])
			reader.Write(temp)
		}
		handler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(OpcodeText, message.Typ())
			as.Equal(expectedText, string(message.Bytes()))
		}

		if err := socket.readMessage(); err != nil {
			t.Fail()
			return
		}
		if err := socket.readMessage(); err != nil {
			t.Fail()
			return
		}
	})

	t.Run("invalid utf8", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var key = internal.NewMaskKey()
		var fh = frameHeader{}
		text, _ := hex.DecodeString("cebae1bdb9cf83cebcceb5eda080656469746564")
		var n = len(text)
		fh.GenerateServerHeader(true, false, OpcodeText, n)
		fh.SetMask()
		fh.SetMaskKey(2, key)
		reader.Write(fh[:6])
		var temp = make([]byte, n)
		copy(temp, text)
		maskXOR(temp, key[0:])
		reader.Write(temp)

		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
		}

		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
	})

	t.Run("close", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		reader.Reset()
		var key = internal.NewMaskKey()
		var fh = frameHeader{}
		fh.GenerateServerHeader(true, false, OpcodeCloseConnection, 2)
		fh.SetMask()
		fh.SetMaskKey(2, key)
		reader.Write(fh[:6])

		var content = internal.CloseProtocolError.Bytes()
		maskXOR(content, key[0:])
		reader.Write(content)

		handler.onClose = func(socket *Conn, code uint16, reason []byte) {
			as.Equal(internal.CloseProtocolError.Uint16(), code)
			as.Equal(0, len(reason))
		}

		err := socket.readMessage()
		as.Error(err)
	})
}

func TestConn_ReadMessageCompress(t *testing.T) {
	var as = assert.New(t)
	var handler = new(webSocketMocker)
	var config = &Config{
		CompressEnabled:   true,
		CheckTextEncoding: true,
	}
	config.initialize()
	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	var socket = serveWebSocket(config, &Request{}, &net.TCPConn{}, brw, handler, true)

	var key = internal.NewMaskKey()
	var fh = frameHeader{}
	var n = internal.AlphabetNumeric.Intn(internal.ThresholdV2 + 1)
	var text = internal.AlphabetNumeric.Generate(n)

	compressedText, err := socket.compressor.Compress(text)
	if err != nil {
		t.Fail()
		return
	}

	var offset = fh.GenerateServerHeader(true, true, OpcodeText, len(compressedText))
	fh.SetMask()
	fh.SetMaskKey(offset, key)
	reader.Write(fh[:offset+4])
	var temp = make([]byte, len(compressedText))
	copy(temp, compressedText)
	maskXOR(temp, key[0:])
	reader.Write(temp)

	handler.onMessage = func(socket *Conn, message *Message) {
		as.Equal(OpcodeText, message.Typ())
		as.Equal(string(text), string(message.Bytes()))
	}

	if err := socket.readMessage(); err != nil {
		t.Fail()
		return
	}
}
