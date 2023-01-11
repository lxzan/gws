package gws

import (
	"bufio"
	"bytes"
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

//go:embed examples/testsuite/config/testdata.json
var testdata []byte

func TestRead(t *testing.T) {
	var as = assert.New(t)
	type Row struct {
		Title    string `json:"title"`
		Fin      bool   `json:"fin"`
		Opcode   int    `json:"opcode"`
		Length   int    `json:"length"`
		Payload  string `json:"payload"`
		Expected struct {
			Event  string `json:"event"`
			Code   uint16 `json:"code"`
			Reason string `json:"reason"`
		} `json:"expected"`
	}

	var items = make([]Row, 0)
	if err := json.Unmarshal(testdata, &items); err != nil {
		as.NoError(err)
		return
	}

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

		if err := handler.writeToReader(socket, reader, Opcode(item.Opcode), payload); err != nil {
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
