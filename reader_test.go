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
	"testing"
)

// 测试同步读
func TestReadSync(t *testing.T) {
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(handler, &ServerOption{
		CompressEnabled:   true,
		CompressThreshold: 512,
	})

	var mu = &sync.Mutex{}
	var listA []string
	var listB []string
	const count = 1000
	var wg = &sync.WaitGroup{}
	wg.Add(count)

	handler.onMessage = func(socket *Conn, message *Message) {
		mu.Lock()
		listB = append(listB, message.Data.String())
		mu.Unlock()
		wg.Done()
	}

	server, client := testNewPeer(upgrader)

	go func() {
		for i := 0; i < count; i++ {
			var n = internal.AlphabetNumeric.Intn(1024)
			var message = internal.AlphabetNumeric.Generate(n)
			listA = append(listA, string(message))
			testClientWrite(client, true, OpcodeText, message)
		}
	}()

	go server.Listen()

	wg.Wait()
	assert.ElementsMatch(t, listA, listB)
}

//go:embed assets/read_test.json
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

	for _, item := range items {
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

		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var handler = new(webSocketMocker)
		var upgrader = NewUpgrader(handler, &ServerOption{
			CompressEnabled:     true,
			CheckUtf8Enabled:    true,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 1024 * 1024,
		})

		switch item.Expected.Event {
		case "onMessage":
			handler.onMessage = func(socket *Conn, message *Message) {
				as.Equal(string(payload), message.Data.String())
				wg.Done()
			}
		case "onPing":
			handler.onPing = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onPong":
			handler.onPong = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onError":
			handler.onError = func(socket *Conn, err error) {
				as.Error(err)
				wg.Done()
			}
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
		}

		server, client := testNewPeer(upgrader)
		buf := bytes.NewBufferString("")
		buf.Write(payload)
		go testClientWrite(client, item.Fin, Opcode(item.Opcode), buf.Bytes())
		go server.Listen()
		wg.Wait()
	}
}

func TestSegments(t *testing.T) {
	var as = assert.New(t)

	t.Run("valid segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var handler = new(webSocketMocker)
		var upgrader = NewUpgrader(handler, nil)

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		handler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(string(s1)+string(s2), message.Data.String())
			wg.Done()
		}

		server, client := testNewPeer(upgrader)
		go func() {
			testClientWrite(client, false, OpcodeText, testCloneBytes(s1))
			testClientWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		go server.Listen()
		wg.Wait()
	})

	t.Run("long segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var handler = new(webSocketMocker)
		var upgrader = NewUpgrader(handler, &ServerOption{
			ReadMaxPayloadSize:  16,
			WriteMaxPayloadSize: 16,
		})

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := testNewPeer(upgrader)
		go func() {
			testClientWrite(client, false, OpcodeText, testCloneBytes(s1))
			testClientWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		go server.Listen()
		wg.Wait()
	})

	t.Run("invalid segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var handler = new(webSocketMocker)
		var upgrader = NewUpgrader(handler, nil)

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := testNewPeer(upgrader)
		go func() {
			testClientWrite(client, false, OpcodeText, testCloneBytes(s1))
			testClientWrite(client, true, OpcodeText, testCloneBytes(s2))
		}()
		go server.Listen()
		wg.Wait()
	})
}

func TestUnexpectedBehavior(t *testing.T) {
	var as = assert.New(t)
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(handler, &ServerOption{})

	var writer = bytes.NewBuffer(nil)
	var reader = bytes.NewBuffer(nil)
	var brw = bufio.NewReadWriter(bufio.NewReader(reader), bufio.NewWriter(writer))
	conn, _ := net.Pipe()
	var socket = serveWebSocket(upgrader.option.ToConfig(), &Request{}, conn, brw, handler, false)
	socket.compressor = newCompressor(flate.BestSpeed)

	t.Run("invalid length 1", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var fh = frameHeader{}
		var key = internal.NewMaskKey()
		var offset = fh.GenerateServerHeader(true, false, OpcodePing, 10)
		fh.SetMask()
		fh.SetMaskKey(offset, key)
		reader.Write(fh[:offset+4])
		var text = internal.AlphabetNumeric.Generate(5)
		internal.MaskXOR(text, key[0:])
		reader.Write(text)

		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
		wg.Wait()
	})

	t.Run("invalid length 2", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var fh = frameHeader{}
		var key = internal.NewMaskKey()
		var offset = fh.GenerateServerHeader(true, false, OpcodePing, 10)
		fh.SetMask()
		fh.SetMaskKey(offset, key)
		reader.Write(fh[:offset])

		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
		wg.Wait()
	})

	t.Run("invalid length 3", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var fh = frameHeader{}
		var key = internal.NewMaskKey()
		var offset = fh.GenerateServerHeader(true, false, OpcodePing, 10)
		fh.SetMask()
		fh.SetMaskKey(offset, key)
		reader.Write(fh[:1])

		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
		wg.Wait()
	})

	t.Run("no mask", func(t *testing.T) {
		handler.reset(socket, reader, writer)
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		var fh = frameHeader{}
		var key = internal.NewMaskKey()
		var offset = fh.GenerateServerHeader(true, false, OpcodePing, 10)
		fh.SetMask()
		fh.SetMaskKey(offset, key)
		reader.Write([]byte{128, 0})

		handler.onError = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		if err := socket.readMessage(); err != nil {
			socket.emitError(err)
		}
		wg.Wait()
	})

	t.Run("illegal rsv", func(t *testing.T) {
		reader.Reset()
		socket.rbuf.Reset(reader)
		reader.Write([]byte{192, 0})
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

func TestMessage(t *testing.T) {
	var msg = &Message{
		Opcode: OpcodeText,
		Data:   bytes.NewBufferString("1234"),
	}
	_, _ = msg.Read(make([]byte, 2))
	msg.Close()
}
