package gws

import (
	"bufio"
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

	t.Run("next call discards unread uncompressed message", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		msg1 := internal.AlphabetNumeric.Generate(64)
		msg2 := internal.AlphabetNumeric.Generate(16)
		go func() {
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg1))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg2))
		}()

		mt1, _, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt1)

		mt2, r2, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt2)
		as.Equal(msg2, readAllReader(t, r2))
	})

	t.Run("next call reports discard error", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() {
			header := frameHeader{}
			headerLength, maskBytes := header.GenerateHeader(false, true, false, OpcodeText, 16)
			payload := internal.AlphabetNumeric.Generate(4)
			internal.MaskXOR(payload, maskBytes)
			_, _ = client.conn.Write(header[:headerLength])
			_, _ = client.conn.Write(payload)
			_ = client.conn.Close()
		}()

		_, _, err := server.NextReader()
		as.NoError(err)
		_, _, err = server.NextReader()
		as.Error(err)
	})

	t.Run("next call closes unread compressed message", func(t *testing.T) {
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

		msg1 := internal.AlphabetNumeric.Generate(2048)
		msg2 := internal.AlphabetNumeric.Generate(128)
		go func() {
			_ = writeFragmentedCompressed(client, OpcodeText, testCloneBytes(msg1))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(msg2))
		}()

		mt1, _, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt1)

		mt2, r2, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt2)
		as.Equal(msg2, readAllReader(t, r2))
	})

	t.Run("next call releases partially read compressed output", func(t *testing.T) {
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

		msg1 := internal.AlphabetNumeric.Generate(2048)
		msg2 := internal.AlphabetNumeric.Generate(128)
		go func() {
			client.WriteAsync(OpcodeText, testCloneBytes(msg1), nil)
			client.WriteAsync(OpcodeText, testCloneBytes(msg2), nil)
		}()

		_, r1, err := server.NextReader()
		as.NoError(err)
		buf := make([]byte, 8)
		n, err := r1.Read(buf)
		as.NoError(err)
		as.Equal(len(buf), n)

		mt2, r2, err := server.NextReader()
		as.NoError(err)
		as.Equal(OpcodeText, mt2)
		as.Equal(msg2, readAllReader(t, r2))
	})

	t.Run("compressed read error propagated", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		serverOption := &ServerOption{PermessageDeflate: PermessageDeflate{Enabled: true}}
		clientOption := &ClientOption{PermessageDeflate: PermessageDeflate{Enabled: true}}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go func() { _, _ = io.Copy(io.Discard, client.NetConn()) }()
		go func() { _ = writeRawFrame(client, OpcodeText, []byte("invalid deflate"), true, true) }()

		_, r, err := server.NextReader()
		as.NoError(err)
		_, err = io.ReadAll(r)
		as.Error(err)
	})

	t.Run("first continuation frame returns protocol error", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() { _, _ = io.Copy(io.Discard, client.NetConn()) }()
		go func() { _ = testWrite(client, true, OpcodeContinuation, []byte("bad")) }()

		_, _, err := server.NextReader()
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("rsv2 returns protocol error", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() { _, _ = io.Copy(io.Discard, client.NetConn()) }()
		go func() {
			payload := []byte("bad")
			header := frameHeader{}
			headerLength, maskBytes := header.GenerateHeader(false, true, false, OpcodeText, len(payload))
			header[0] |= 0x20
			internal.MaskXOR(payload, maskBytes)
			_, _ = client.conn.Write(header[:headerLength])
			_, _ = client.conn.Write(payload)
		}()

		_, _, err := server.NextReader()
		as.Equal(internal.CloseProtocolError, err)
	})
}

func TestConn_ReadMessageManual(t *testing.T) {
	var as = assert.New(t)

	t.Run("returns complete message", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		payload := internal.AlphabetNumeric.Generate(32)
		go func() { _ = testWrite(client, true, OpcodeText, testCloneBytes(payload)) }()

		msg, err := server.ReadMessage()
		as.NoError(err)
		defer msg.Close()
		as.Equal(OpcodeText, msg.Opcode)
		as.Equal(payload, msg.Bytes())
	})

	t.Run("skips control frames", func(t *testing.T) {
		pingDone := make(chan struct{}, 1)
		serverHandler := &webSocketMocker{
			onPing: func(socket *Conn, payload []byte) {
				pingDone <- struct{}{}
			},
		}
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		payload := internal.AlphabetNumeric.Generate(32)
		go func() {
			_ = testWrite(client, true, OpcodePing, []byte("ping"))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(payload))
		}()

		msg, err := server.ReadMessage()
		as.NoError(err)
		defer msg.Close()
		as.Equal(payload, msg.Bytes())
		select {
		case <-pingDone:
		default:
			t.Fatal("expected ping callback")
		}
	})

	t.Run("handles read error", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		_ = client.NetConn().Close()

		msg, err := server.ReadMessage()
		as.Nil(msg)
		as.Error(err)
	})
}

func TestConn_ReadDataFrameHeader(t *testing.T) {
	var as = assert.New(t)

	t.Run("skips control frame", func(t *testing.T) {
		pingDone := make(chan struct{}, 1)
		serverHandler := &webSocketMocker{
			onPing: func(socket *Conn, payload []byte) {
				pingDone <- struct{}{}
			},
		}
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})

		payload := internal.AlphabetNumeric.Generate(8)
		go func() {
			_ = testWrite(client, true, OpcodePing, []byte("ping"))
			_ = testWrite(client, true, OpcodeText, testCloneBytes(payload))
		}()

		h, n, err := server.readDataFrameHeader(false)
		as.NoError(err)
		as.Equal(OpcodeText, h.GetOpcode())
		as.Equal(len(payload), n)
		select {
		case <-pingDone:
		default:
			t.Fatal("expected ping callback")
		}
	})

	t.Run("message too large", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{ReadMaxPayloadSize: 4}, clientHandler, &ClientOption{})
		go func() { _ = testWrite(client, true, OpcodeText, internal.AlphabetNumeric.Generate(8)) }()

		h, n, err := server.readDataFrameHeader(false)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseMessageTooLarge, err)
	})

	t.Run("rejects unmasked client frame", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() {
			header := frameHeader{}
			headerLength, _ := header.GenerateHeader(true, true, false, OpcodeText, 0)
			_, _ = client.conn.Write(header[:headerLength])
		}()

		h, n, err := server.readDataFrameHeader(false)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("control frame error", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() { _ = testWrite(client, false, OpcodePing, nil) }()

		h, n, err := server.readDataFrameHeader(false)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("expect continuation rejects data frame", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() { _ = testWrite(client, true, OpcodeText, []byte("bad")) }()

		h, n, err := server.readDataFrameHeader(true)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("expect continuation rejects rsv1", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{PermessageDeflate: PermessageDeflate{Enabled: true}}, clientHandler, &ClientOption{})
		go func() { _ = writeRawFrame(client, OpcodeContinuation, []byte("bad"), true, true) }()

		h, n, err := server.readDataFrameHeader(true)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("rejects rsv2", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{PermessageDeflate: PermessageDeflate{Enabled: true}}, clientHandler, &ClientOption{})
		go func() {
			payload := []byte("bad")
			header := frameHeader{}
			headerLength, maskBytes := header.GenerateHeader(false, true, false, OpcodeText, len(payload))
			header[0] |= 0x20
			internal.MaskXOR(payload, maskBytes)
			_, _ = client.conn.Write(header[:headerLength])
			_, _ = client.conn.Write(payload)
		}()

		h, n, err := server.readDataFrameHeader(false)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseProtocolError, err)
	})

	t.Run("rejects rsv3", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{PermessageDeflate: PermessageDeflate{Enabled: true}}, clientHandler, &ClientOption{})
		go func() {
			payload := []byte("bad")
			header := frameHeader{}
			headerLength, maskBytes := header.GenerateHeader(false, true, false, OpcodeText, len(payload))
			header[0] |= 0x10
			internal.MaskXOR(payload, maskBytes)
			_, _ = client.conn.Write(header[:headerLength])
			_, _ = client.conn.Write(payload)
		}()

		h, n, err := server.readDataFrameHeader(false)
		as.Nil(h)
		as.Equal(0, n)
		as.Equal(internal.CloseProtocolError, err)
	})
}

func TestNextMessageReaderInternals(t *testing.T) {
	var as = assert.New(t)

	cfg := initServerOption(&ServerOption{ReadMaxPayloadSize: 4}).getConfig()
	conn := &Conn{config: cfg}

	t.Run("uncompressed guards", func(t *testing.T) {
		r := &uncompressedMessageReader{conn: conn, streamDone: true}
		n, err := r.Read(make([]byte, 1))
		as.Equal(0, n)
		as.Equal(io.EOF, err)
		as.NoError(r.close())

		r = &uncompressedMessageReader{messageDone: true}
		n, err = r.read(make([]byte, 1))
		as.Equal(0, n)
		as.Equal(io.EOF, err)

		r = &uncompressedMessageReader{conn: conn}
		as.NoError(r.trackPayload(0))
		as.NoError(r.failRead(nil))

		n, err = r.read(nil)
		as.Equal(0, n)
		as.NoError(err)
	})

	t.Run("uncompressed zero length fragment continues", func(t *testing.T) {
		serverHandler := new(webSocketMocker)
		clientHandler := new(webSocketMocker)
		server, client := newPeer(serverHandler, &ServerOption{}, clientHandler, &ClientOption{})
		go func() {
			_ = testWrite(client, false, OpcodeText, nil)
			_ = testWrite(client, false, OpcodeContinuation, nil)
			_ = testWrite(client, true, OpcodeContinuation, []byte("ok"))
		}()

		_, r, err := server.NextReader()
		as.NoError(err)
		as.Equal([]byte("ok"), readAllReader(t, r))
	})

	t.Run("message guards", func(t *testing.T) {
		r := &messageReader{conn: conn, streamDone: true}
		n, err := r.Read(make([]byte, 1))
		as.Equal(0, n)
		as.Equal(io.EOF, err)
		as.NoError(r.close())

		r = &messageReader{conn: conn}
		as.NoError(r.trackPayload(nil))
		as.Equal(internal.CloseMessageTooLarge, r.trackPayload([]byte("12345")))
		as.NoError(r.failRead(nil))

		r.compressedBuf = binaryPool.Get(1)
		r.releaseCompressed()
		as.Nil(r.compressedBuf)
	})

	t.Run("compressed header error while filling", func(t *testing.T) {
		r := &messageReader{
			conn: &Conn{
				br:     bufio.NewReader(bytes.NewReader(nil)),
				config: cfg,
			},
		}
		err := r.fillCompressedOutput()
		as.Error(err)
		r.releaseCompressed()
	})

	t.Run("compressed read error while filling", func(t *testing.T) {
		r := &messageReader{
			conn: &Conn{
				br:     bufio.NewReader(bytes.NewReader([]byte("short"))),
				config: cfg,
			},
			framePayload: 5000,
			frameFIN:     true,
		}
		err := r.fillCompressedOutput()
		as.Error(err)
		r.releaseCompressed()
	})

	t.Run("compressed payload too large while filling", func(t *testing.T) {
		smallCfg := initServerOption(&ServerOption{ReadMaxPayloadSize: 1}).getConfig()
		r := &messageReader{
			conn: &Conn{
				br:     bufio.NewReader(bytes.NewReader([]byte("12"))),
				config: smallCfg,
			},
			framePayload: 2,
			frameFIN:     true,
		}
		err := r.fillCompressedOutput()
		as.Equal(internal.CloseMessageTooLarge, err)
		r.releaseCompressed()
	})

	t.Run("normalize nil error", func(t *testing.T) {
		as.NoError(normalizeReadError(nil))
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
