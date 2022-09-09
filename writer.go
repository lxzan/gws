package websocket

import (
	"bytes"
)

func (c *Conn) WritePing(payload []byte) error {
	return c.writeFrame(Opcode_Ping, payload, false)
}

func (c *Conn) WritePong(payload []byte) error {
	return c.writeFrame(Opcode_Pong, payload, false)
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
		return c.writeFrame(opcode, content, enableCompress)
	}

	var compressor = c.compressors.Select()
	compressedContent, err := compressor.Compress(content)
	defer compressor.Close()
	if err != nil {
		return err
	}
	return c.writeFrame(opcode, compressedContent, enableCompress)
}

// write a websocket frame, content is prepared
func (c *Conn) writeFrame(opcode Opcode, payload []byte, enableCompress bool) error {
	var header = frameHeader{}
	var n = len(payload)
	var headerLength = header.GenerateServerHeader(opcode, enableCompress, n)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := writeN(c.netConn, header[:headerLength], headerLength); err != nil {
		return err
	}
	if n > 0 {
		if err := writeN(c.netConn, payload, n); err != nil {
			return err
		}
	}
	return nil
}
