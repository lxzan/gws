package gws

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

// readAllReader reads all bytes from a reader.
func readAllReader(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	assert.NoError(t, err)
	return b
}

func TestConn_NextReader(t *testing.T) {
	var as = assert.New(t)

	t.Run("returns one message per call", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		msg1 := internal.AlphabetNumeric.Generate(10)
		msg2 := internal.AlphabetNumeric.Generate(10)

		go func() {
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg1))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg2))
		}()

		mt1, r1, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt1)
		got1 := readAllReader(t, r1)
		as.Equal(string(msg1), string(got1))

		mt2, r2, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt2)
		got2 := readAllReader(t, r2)
		as.Equal(string(msg2), string(got2))
	})

	t.Run("does not wait for next message", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		msg1 := internal.AlphabetNumeric.Generate(10)

		go func() { _ = testWrite(client, true, OpcodeText, testCloneBytes(msg1)) }()

		done := make(chan struct{})
		var messageType Opcode
		var got []byte
		var err error
		go func() {
			var r io.Reader
			messageType, r, err = server.NextReader()
			if err == nil {
				got = readAllReader(t, r)
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Read blocked waiting for a message that was never sent")
		}

		as.NoError(err)
		as.Equal(OpcodeText, messageType)
		as.Equal(string(msg1), string(got))
	})

	t.Run("fragmented message reassembled transparently", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		want := string(s1) + string(s2)

		go func() {
			_ = testWrite(client, false, OpcodeText, testCloneBytes(s1))
			_ = testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()

		messageType, r, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, messageType)
		got := readAllReader(t, r)
		as.Equal(want, string(got))
	})

	t.Run("does not validate utf8", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{CheckUtf8Enabled: true}, clientHandler, &ClientOption{})

		payload := []byte{0xff, 0xfe}
		go func() { _ = writeRawFrame(client, OpcodeText, testCloneBytes(payload), true, false) }()

		messageType, r, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, messageType)
		as.Equal(payload, readAllReader(t, r))
	})

	t.Run("compressed message does not validate utf8", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{
			CheckUtf8Enabled: true,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		}
		clientOption := &ClientOption{
			CheckUtf8Enabled: true,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		payload := []byte{0xff, 0xfe}
		go func() { _ = writeFragmentedCompressed(client, OpcodeText, testCloneBytes(payload)) }()

		messageType, r, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, messageType)
		as.Equal(payload, readAllReader(t, r))
	})

	t.Run("compressed message with context takeover", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: false,
			ServerMaxWindowBits:   10,
			ClientMaxWindowBits:   10,
		}}
		clientOption := &ClientOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		}}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		msg1 := internal.AlphabetNumeric.Generate(2048)
		msg2 := internal.AlphabetNumeric.Generate(1024)
		go func() {
			client.WriteAsync(OpcodeText, testCloneBytes(msg1), nil)
			client.WriteAsync(OpcodeText, testCloneBytes(msg2), nil)
		}()

		mt1, r1, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt1)
		as.Equal(string(msg1), string(readAllReader(t, r1)))

		mt2, r2, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt2)
		as.Equal(string(msg2), string(readAllReader(t, r2)))
	})

	t.Run("read error propagated", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		_ = client.NetConn().Close()

		_, _, err := server.NextReader()
		as.Error(err)
		_, ok := err.(internal.StatusCode)
		as.True(ok)
		as.Equal(internal.CloseAbnormalClosure, err)
	})

	t.Run("invalid fragmented frame returns protocol error", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() { _, _ = io.Copy(io.Discard, client.NetConn()) }()

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		go func() {
			_ = testWrite(client, false, OpcodeText, testCloneBytes(s1))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(s2))
		}()

		_, r, err := server.NextReader()
		as.NoError(err)
		buf := make([]byte, 32)
		n, err := r.Read(buf)
		as.NoError(err)
		as.Equal(len(s1), n)
		_, err = r.Read(buf)
		as.Error(err)
		_, ok := err.(internal.StatusCode)
		as.True(ok)
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("fragmented message too large returns status code", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{ReadMaxPayloadSize: 16}
		server, client := newPeer(serverHandler, serverOption, clientHandler, &ClientOption{})
		go func() { _, _ = io.Copy(io.Discard, client.NetConn()) }()

		s1 := internal.AlphabetNumeric.Generate(16)
		s2 := internal.AlphabetNumeric.Generate(16)
		go func() {
			_ = testWrite(client, false, OpcodeText, testCloneBytes(s1))
			_ = testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()

		_, r, err := server.NextReader()
		as.NoError(err)
		buf := make([]byte, 32)
		n, err := r.Read(buf)
		as.NoError(err)
		as.Equal(len(s1), n)
		_, err = r.Read(buf)
		as.Error(err)
		_, ok := err.(internal.StatusCode)
		as.True(ok)
		as.Equal(internal.CloseMessageTooLarge, err)
	})

	t.Run("fragmented compressed message", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		}}
		clientOption := &ClientOption{PermessageDeflate: PermessageDeflate{
			Enabled:               true,
			ServerContextTakeover: true,
			ClientContextTakeover: true,
		}}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		payload := internal.AlphabetNumeric.Generate(2048)
		go func() { _ = writeFragmentedCompressed(client, OpcodeText, testCloneBytes(payload)) }()

		messageType, r, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, messageType)
		as.Equal(string(payload), string(readAllReader(t, r)))
	})
}

func writeFragmentedCompressed(c *Conn, opcode Opcode, payload []byte) error {
	var buf = bytes.NewBuffer(nil)
	if err := c.deflater.Compress(internal.Bytes(payload), buf, c.cpsWindow.dict); err != nil {
		return err
	}
	compressed := buf.Bytes()
	if len(compressed) < 2 {
		return testWrite(c, true, opcode, payload)
	}
	mid := len(compressed) / 2
	if err := writeRawFrame(c, opcode, compressed[:mid], false, true); err != nil {
		return err
	}
	return writeRawFrame(c, OpcodeContinuation, compressed[mid:], true, false)
}

func writeRawFrame(c *Conn, opcode Opcode, payload []byte, fin bool, compress bool) error {
	var header = frameHeader{}
	headerLength, maskBytes := header.GenerateHeader(c.isServer, fin, compress, opcode, len(payload))
	if len(payload) > 0 && !c.isServer {
		internal.MaskXOR(payload, maskBytes)
	}
	var frame = make(net.Buffers, 0, 2)
	frame = append(frame, header[:headerLength])
	if len(payload) > 0 {
		frame = append(frame, payload)
	}
	_, err := frame.WriteTo(c.conn)
	return err
}
