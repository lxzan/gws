package websocket

import (
	"bytes"
	"github.com/lxzan/gws/internal"
	"io"
)

func (c *Conn) WritePing() error {
	c.mu.Lock()
	num, err := c.netConn.Write(internal.PingFrame)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if num != 3 {
		return CloseNormalClosure
	}
	return nil
}

func (c *Conn) WritePong() error {
	c.mu.Lock()
	num, err := c.netConn.Write(internal.PongFrame)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if num != 3 {
		return CloseNormalClosure
	}
	return nil
}

func (c *Conn) Write(opcode Opcode, content []byte) error {
	err := c.writeMessage(opcode, content)
	_pool.Put(bytes.NewBuffer(content))
	if err != nil {
		c.emitError(err)
	}
	return err
}

// 加锁是为了防止frame header和payload并发写入后乱序
func (c *Conn) writeMessage(opcode Opcode, content []byte) error {
	var enableCompress = c.compress && isDataFrame(opcode)
	if !enableCompress {
		n := len(content)
		header, headerLength := genHeader(c.side, opcode, true, true, enableCompress, uint64(n))
		c.mu.Lock()
		defer c.mu.Unlock()
		if err := writeN(c.netConn, header[:headerLength], headerLength); err != nil {
			return err
		}
		if _, err := io.CopyN(c.netConn, bytes.NewBuffer(content), int64(n)); err != nil {
			return err
		}
	} else {
		var compressor = c.compressors.Select()
		c.mu.Lock()
		result, err := compressor.Compress(content)
		defer func() {
			compressor.Unlock()
			c.mu.Unlock()
		}()

		if err != nil {
			return err
		}
		header, headerLength := genHeader(c.side, opcode, true, true, enableCompress, uint64(len(result)))
		if err := writeN(c.netConn, header[:headerLength], headerLength); err != nil {
			return err
		}
		if _, err := io.CopyN(c.netConn, bytes.NewBuffer(result), int64(len(result))); err != nil {
			return err
		}
	}

	return nil
}
