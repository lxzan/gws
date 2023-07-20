package gws

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"math"
	"sync"
	"sync/atomic"
)

type (
	Broadcaster struct {
		opcode  Opcode
		payload []byte
		msgs    [2]*broadcastMessageWrapper
		state   atomic.Int64
	}

	broadcastMessageWrapper struct {
		once  sync.Once
		err   error
		index int
		frame *bytes.Buffer
	}
)

// NewBroadcaster
func NewBroadcaster(opcode Opcode, payload []byte) *Broadcaster {
	c := &Broadcaster{
		opcode:  opcode,
		payload: payload,
		msgs:    [2]*broadcastMessageWrapper{},
	}
	c.state.Add(math.MaxInt32)
	return c
}

// Broadcast 广播
// 不要并行调用Broadcast方法
func (c *Broadcaster) Broadcast(socket *Conn) error {
	var idx = internal.SelectValue(socket.compressEnabled, 1, 0)
	var msg = c.msgs[idx]
	if msg == nil {
		c.msgs[idx] = &broadcastMessageWrapper{}
		msg = c.msgs[idx]
		msg.frame, msg.index, msg.err = socket.genFrame(c.opcode, c.payload)
	}
	if msg.err != nil {
		return msg.err
	}

	c.state.Add(1)
	socket.writeQueue.Push(func() {
		if !socket.isClosed() {
			socket.emitError(internal.WriteN(socket.conn, msg.frame.Bytes(), msg.frame.Len()))
		}
		if c.state.Add(-1) == 0 {
			c.doClose()
		}
	})
	return nil
}

func (c *Broadcaster) doClose() {
	for _, item := range c.msgs {
		if item != nil {
			myBufferPool.Put(item.frame, item.index)
		}
	}
}

func (c *Broadcaster) Release() {
	if c.state.Add(-1*math.MaxInt32) == 0 {
		c.doClose()
	}
}
