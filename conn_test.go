package gws

import (
	"bytes"
	"errors"
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
	onOpen    func(socket *Conn)
}

func (c *webSocketMocker) OnOpen(socket *Conn) {
	if c.onOpen != nil {
		c.onOpen(socket)
	}
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

func TestBuiltinEventHandler(t *testing.T) {
	handler := BuiltinEventHandler{}
	handler.OnOpen(nil)
	handler.OnClose(nil, nil)
	handler.OnPong(nil, nil)
	handler.OnMessage(nil, nil)
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
}

func TestConn_Close(t *testing.T) {
	conn, _ := net.Pipe()
	var options = initServerOption(nil)
	var socket = &Conn{conn: conn, config: options.getConfig()}
	_ = conn.Close()
	assert.Error(t, socket.SetDeadline(time.Time{}))
	assert.Error(t, socket.SetReadDeadline(time.Time{}))
	assert.Error(t, socket.SetWriteDeadline(time.Time{}))
}

func TestConn_SubProtocol(t *testing.T) {
	conn := new(Conn)
	conn.SubProtocol()
}

// 钉住零值安全: &Conn{}字面量绕过构造器, 随机源状态为零, 必须不卡死不恐慌且掩码键可用.
// Pins zero-value safety: a &Conn{} literal bypasses the constructor and its random
// state is zero; nextMaskKey must still work without hanging or panicking.
func TestConn_NextMaskKey_ZeroValue(t *testing.T) {
	var as = assert.New(t)
	var conn = &Conn{}
	var seen = make(map[uint32]struct{}, 64)
	for range 64 {
		seen[conn.nextMaskKey()] = struct{}{}
	}
	as.Equal(64, len(seen))
}

func TestConn_NextMaskKey_ServerReturnsZero(t *testing.T) {
	var conn = &Conn{isServer: true}
	for range 10 {
		assert.EqualValues(t, 0, conn.nextMaskKey())
	}
}

// 广播路径生成帧时不持有连接锁, 随机源的并发安全由原子操作保证 (-race 校验).
// Broadcast frame generation runs without the connection lock, so the random
// source must stay safe under concurrent access (verified with -race).
func TestConn_NextMaskKey_Concurrent(t *testing.T) {
	var conn = &Conn{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10000 {
				_ = conn.nextMaskKey()
			}
		}()
	}
	wg.Wait()
}

// 钉住32位平台对齐修复: randState 必须保持 atomic.Uint64. 该类型内含 align64,
// 编译器保证字段在任意偏移处8字节对齐, 32位平台(386/arm/mips32)的64位原子操作不会恐慌;
// 裸 uint64 落在4字节对齐偏移上会在运行时触发 unaligned 64-bit atomic panic.
// Pins the 32-bit alignment fix: randState must stay atomic.Uint64, whose embedded align64
// makes the compiler guarantee an 8-byte aligned offset at any struct position, so 64-bit
// atomics never panic on 32-bit platforms (386/arm/mips32); a plain uint64 would not.
func TestConn_RandStateAlignment(t *testing.T) {
	var conn Conn
	var _ *atomic.Uint64 = &conn.randState
	assert.IsType(t, &atomic.Uint64{}, &conn.randState)
}

func TestConn_ZeroValueWriteMessage(t *testing.T) {
	var upgrader = NewUpgrader(&BuiltinEventHandler{}, nil)
	var conn = &Conn{conn: &benchConn{}, config: upgrader.option.getConfig()}
	for range 100 {
		assert.NoError(t, conn.WriteMessage(OpcodeText, []byte("hello")))
	}
}

func TestConn_EmitClose(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{CheckUtf8Enabled: true}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		clientHandler.onClose = func(socket *Conn, err error) {
			if err.(*CloseError).Code == internal.CloseProtocolError.Uint16() {
				wg.Done()
			}
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		server.emitClose(bytes.NewBuffer(internal.StatusCode(500).Bytes()))
		wg.Wait()
	})

	t.Run("", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{CheckUtf8Enabled: true}
		var clientOption = &ClientOption{}
		var wg = &sync.WaitGroup{}
		wg.Add(1)
		clientHandler.onClose = func(socket *Conn, err error) {
			if err.(*CloseError).Code == 4000 {
				wg.Done()
			}
		}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		server.emitClose(bytes.NewBuffer(internal.StatusCode(4000).Bytes()))
		wg.Wait()
	})
}

// OnOpen回调panic必须被Recovery拦截并关闭当前连接, 否则用户回调panic会击穿进程
// A panic in the OnOpen callback must be trapped by Recovery and the connection closed,
// otherwise a user callback panic crashes the whole process
func TestConn_OnOpenPanic(t *testing.T) {
	var serverHandler = new(webSocketMocker)
	serverHandler.onOpen = func(socket *Conn) { panic("boom") }
	var clientHandler = new(webSocketMocker)
	var closeCh = make(chan error, 1)
	clientHandler.onClose = func(socket *Conn, err error) { closeCh <- err }

	var addr = ":" + nextPort()
	var server = NewServer(serverHandler, nil)
	go server.Run(addr)
	_ = waitServerReady(t, "localhost"+addr).Close()

	var client = dialWithRetry(t, clientHandler, &ClientOption{Addr: "ws://localhost" + addr})
	go client.ReadLoop()

	select {
	case <-closeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("connection not closed after OnOpen panic")
	}
}

func TestConn_EmitError(t *testing.T) {
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CheckUtf8Enabled: true}
	var clientOption = &ClientOption{}
	var wg = &sync.WaitGroup{}
	wg.Add(1)
	clientHandler.onClose = func(socket *Conn, err error) {
		wg.Done()
	}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go client.ReadLoop()
	err := errors.New(string(internal.AlphabetNumeric.Generate(500)))
	server.emitError(false, err)
	wg.Wait()
}
