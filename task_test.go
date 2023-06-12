package gws

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

func newPeer(serverHandler Event, serverOption *ServerOption, clientHandler Event, clientOption *ClientOption) (server, client *Conn) {
	serverOption = initServerOption(serverOption)
	clientOption = initClientOption(clientOption)
	size := 4096
	s, c := net.Pipe()
	{
		br := bufio.NewReaderSize(s, size)
		server = serveWebSocket(true, serverOption.getConfig(), new(sliceMap), s, br, serverHandler, serverOption.CompressEnabled)
	}
	{
		br := bufio.NewReaderSize(c, size)
		client = serveWebSocket(false, clientOption.getConfig(), new(sliceMap), c, br, clientHandler, clientOption.CompressEnabled)
	}
	return
}

func testCloneBytes(b []byte) []byte {
	p := make([]byte, len(b))
	copy(p, b)
	return p
}

// 测试异步写入
func TestConn_WriteAsync(t *testing.T) {
	var as = assert.New(t)

	// 关闭压缩
	t.Run("plain text", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		var listA []string
		var listB []string
		var count = 128
		var wg sync.WaitGroup
		wg.Add(count)
		clientHandler.onMessage = func(socket *Conn, message *Message) {
			listB = append(listB, message.Data.String())
			wg.Done()
		}

		go server.ReadLoop()
		go client.ReadLoop()
		for i := 0; i < count; i++ {
			var n = internal.AlphabetNumeric.Intn(125)
			var message = internal.AlphabetNumeric.Generate(n)
			listA = append(listA, string(message))
			server.WriteAsync(OpcodeText, message)
		}
		wg.Wait()
		as.ElementsMatch(listA, listB)
	})

	// 开启压缩
	t.Run("compressed text", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{CompressEnabled: true, CompressThreshold: 1}
		var clientOption = &ClientOption{CompressEnabled: true, CompressThreshold: 1}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		var listA []string
		var listB []string
		const count = 128
		var wg sync.WaitGroup
		wg.Add(count)

		clientHandler.onMessage = func(socket *Conn, message *Message) {
			listB = append(listB, message.Data.String())
			wg.Done()
		}

		go server.ReadLoop()
		go client.ReadLoop()
		go func() {
			for i := 0; i < count; i++ {
				var n = internal.AlphabetNumeric.Intn(1024)
				var message = internal.AlphabetNumeric.Generate(n)
				listA = append(listA, string(message))
				server.WriteAsync(OpcodeText, message)
			}
		}()

		wg.Wait()
		as.ElementsMatch(listA, listB)
	})

	// 往关闭的连接写数据
	t.Run("write to closed conn", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		var wg = sync.WaitGroup{}
		wg.Add(1)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}
		client.NetConn().Close()
		server.WriteAsync(OpcodeText, internal.AlphabetNumeric.Generate(8))
		wg.Wait()
	})

	t.Run("ping/pong", func(t *testing.T) {
		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}
		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

		var wg = sync.WaitGroup{}
		wg.Add(4)

		serverHandler.onPing = func(socket *Conn, payload []byte) {
			wg.Done()
			socket.WritePong(nil)
		}
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			if string(message.Bytes()) == "hello" {
				wg.Done()
			}
		}
		serverHandler.onClose = func(socket *Conn, err error) {
			wg.Done()
		}
		clientHandler.onPong = func(socket *Conn, payload []byte) {
			wg.Done()
		}

		go server.ReadLoop()
		go client.ReadLoop()
		client.WritePing(nil)
		client.WriteString("hello")

		{
			var fh = frameHeader{}
			var n, _ = fh.GenerateHeader(true, true, false, OpcodeText, 0)
			go func() { client.conn.Write(fh[:n]) }()
		}

		wg.Wait()
	})
}

// 测试异步读
func TestReadAsync(t *testing.T) {
	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{CompressEnabled: true, CompressThreshold: 512, ReadAsyncEnabled: true}
	var clientOption = &ClientOption{CompressEnabled: true, CompressThreshold: 512, ReadAsyncEnabled: true}
	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)

	var mu = &sync.Mutex{}
	var listA []string
	var listB []string
	const count = 1000
	var wg = &sync.WaitGroup{}
	wg.Add(count)

	clientHandler.onMessage = func(socket *Conn, message *Message) {
		mu.Lock()
		listB = append(listB, message.Data.String())
		mu.Unlock()
		wg.Done()
	}

	go server.ReadLoop()
	go client.ReadLoop()
	for i := 0; i < count; i++ {
		var n = internal.AlphabetNumeric.Intn(1024)
		var message = internal.AlphabetNumeric.Generate(n)
		listA = append(listA, string(message))
		server.WriteMessage(OpcodeText, message)
	}

	wg.Wait()
	assert.ElementsMatch(t, listA, listB)
}

func TestTaskQueue(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var mu = &sync.Mutex{}
		var listA []int
		var listB []int

		var count = 1000
		var wg = &sync.WaitGroup{}
		wg.Add(count)
		var q = newWorkerQueue(8)
		for i := 0; i < count; i++ {
			listA = append(listA, i)

			v := i
			q.Push(func() {
				defer wg.Done()
				var latency = time.Duration(internal.AlphabetNumeric.Intn(100)) * time.Microsecond
				time.Sleep(latency)
				mu.Lock()
				listB = append(listB, v)
				mu.Unlock()
			})
		}
		wg.Wait()
		as.ElementsMatch(listA, listB)
	})

	t.Run("", func(t *testing.T) {
		sum := int64(0)
		w := newWorkerQueue(8)
		var wg = &sync.WaitGroup{}
		wg.Add(1000)
		for i := int64(1); i <= 1000; i++ {
			var tmp = i
			w.Push(func() {
				atomic.AddInt64(&sum, tmp)
				wg.Done()
			})
		}
		wg.Wait()
		as.Equal(sum, int64(500500))
	})
}

func TestWriteAsyncBlocking(t *testing.T) {
	var handler = new(webSocketMocker)
	var upgrader = NewUpgrader(handler, nil)

	allConns := map[*Conn]struct{}{}
	for i := 0; i < 3; i++ {
		svrConn, cliConn := net.Pipe() // no reading from another side
		var sbrw = bufio.NewReader(svrConn)
		var svrSocket = serveWebSocket(true, upgrader.option.getConfig(), &sliceMap{}, svrConn, sbrw, handler, false)
		go svrSocket.ReadLoop()
		var cbrw = bufio.NewReader(cliConn)
		var cliSocket = serveWebSocket(false, upgrader.option.getConfig(), &sliceMap{}, cliConn, cbrw, handler, false)
		if i == 0 { // client 0 1s后再开始读取；1s内不读取消息，则svrSocket 0在发送chan取出一个msg进行writePublic时即开始阻塞
			time.AfterFunc(time.Second, func() {
				cliSocket.ReadLoop()
			})
		} else {
			go cliSocket.ReadLoop()
		}
		allConns[svrSocket] = struct{}{}
	}

	// 第一个msg被异步协程从chan取出了，取出后阻塞在writePublic、没有后续的取出，再入defaultAsyncIOGoLimit个msg到chan里，
	// 则defaultAsyncIOGoLimit+2个消息会导致入chan阻塞。
	// 1s后client 0开始读取，广播才会继续，这一轮对应的时间约为1s
	for i := 0; i <= defaultReadAsyncGoLimit+2; i++ {
		t0 := time.Now()
		for wsConn := range allConns {
			wsConn.WriteAsync(OpcodeBinary, []byte{0})
		}
		fmt.Printf("broadcast %d, used: %v\n", i, time.Since(t0).Nanoseconds())
	}

	time.Sleep(time.Second * 2)
}
