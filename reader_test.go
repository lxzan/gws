package gws

import (
	"bufio"
	"bytes"
	"compress/flate"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"sync/atomic"
	"testing"
)

//go:embed examples/testsuite/config/readtest.json
var testdata []byte

type testRow struct {
	Title    string `json:"title"`
	Fin      bool   `json:"fin"`
	Opcode   uint8  `json:"opcode"`
	Length   int    `json:"length"`
	Payload  string `json:"payload"`
	RSV2     bool   `json:"rsv2"`
	Expected struct {
		Event  string `json:"event"`
		Code   uint16 `json:"code"`
		Reason string `json:"reason"`
	} `json:"expected"`
}

func TestRead(t *testing.T) {
	var as = assert.New(t)
	var items = make([]testRow, 0)
	if err := json.Unmarshal(testdata, &items); err != nil {
		as.NoError(err)
		return
	}

	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(func(c *Upgrader) {
		c.CompressEnabled = true
		c.CheckTextEncoding = true
		c.MaxContentLength = 128 * 1024
		c.EventHandler = handler
	})

	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	var socket = serveWebSocket(upgrader, &Request{}, &net.TCPConn{}, brw, upgrader.EventHandler, true)

	for _, item := range items {
		reader.Reset()
		socket.rbuf.Reset(reader)
		atomic.StoreUint32(&socket.closed, 0)

		var payload []byte
		if item.Payload == "" {
			payload = internal.AlphabetNumeric.Generate(item.Length)
		} else {
			p, err := hex.DecodeString(item.Payload)
			if err != nil {
				as.NoError(err)
				return
			}
			payload = p
		}

		if err := handler.writeToReader(socket, reader, item, payload); err != nil {
			as.NoError(err, item.Title)
			return
		}

		var wg = &sync.WaitGroup{}
		wg.Add(1)

		switch item.Expected.Event {
		case "onMessage":
			handler.onMessage = func(socket *Conn, message *Message) {
				as.Equal(string(payload), string(message.Bytes()))
				go func() { wg.Done() }()
			}
			as.NoError(socket.readMessage())
		case "onPing":
			handler.onPing = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				go func() { wg.Done() }()
			}
			as.NoError(socket.readMessage())
		case "onPong":
			handler.onPong = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				go func() { wg.Done() }()
			}
			as.NoError(socket.readMessage())
		case "onError":
			handler.onError = func(socket *Conn, err error) {
				as.Error(err)
				go func() { wg.Done() }()
			}
			socket.emitError(socket.readMessage())
		case "onClose":
			handler.onClose = func(socket *Conn, code uint16, reason []byte) {
				defer wg.Done()
				as.Equal(item.Expected.Code, code)
				p, err := hex.DecodeString(item.Expected.Reason)
				if err != nil {
					as.NoError(err)
					return
				}
				as.Equal(string(reason), string(p))
			}
			as.Error(socket.readMessage())
		default:
			wg.Done()
		}

		wg.Wait()
	}
}

func TestSegments(t *testing.T) {
	var as = assert.New(t)
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(
		WithEventHandler(handler),
		WithCompress(false, 0),
		WithResponseHeader(nil),
		WithMaxContentLength(0),
		WithCheckTextEncoding(false),
		WithCheckOrigin(func(r *Request) bool {
			return true
		}),
	)
	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	var socket = serveWebSocket(upgrader, &Request{}, &net.TCPConn{}, brw, handler, false)
	socket.compressor = newCompressor(flate.BestSpeed)

	t.Run("valid segments", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		_ = handler.writeToReader(socket, reader, testRow{
			Fin:     false,
			Opcode:  uint8(OpcodeText),
			Payload: hex.EncodeToString(s1),
		}, s1)
		_ = handler.writeToReader(socket, reader, testRow{
			Fin:     true,
			Opcode:  uint8(OpcodeContinuation),
			Payload: hex.EncodeToString(s2),
		}, s2)

		handler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(string(s1)+string(s2), string(message.Bytes()))
			wg.Done()
		}

		_ = socket.readMessage()
		_ = socket.readMessage()
		wg.Wait()
	})

	t.Run("invalid segments", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)

		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		_ = handler.writeToReader(socket, reader, testRow{
			Fin:     false,
			Opcode:  uint8(OpcodeText),
			Payload: hex.EncodeToString(s1),
		}, s1)
		_ = handler.writeToReader(socket, reader, testRow{
			Fin:     true,
			Opcode:  uint8(OpcodeText),
			Payload: hex.EncodeToString(s2),
		}, s2)

		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
		wg.Wait()
	})

	t.Run("illegal rsv", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)
		reader.Write([]byte{127, 0})
		as.Error(socket.readMessage())
	})

	t.Run("no mask", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)
		reader.Write([]byte{128, 0})
		as.Error(socket.readMessage())
	})

	t.Run("eof", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)
		as.Error(socket.readMessage())

		reader.Reset()
		socket.rbuf.Reset(reader)
		reader.Write([]byte{127})
		as.Error(socket.readMessage())
	})
}
