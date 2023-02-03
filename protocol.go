package gws

type Opcode uint8

const (
	OpcodeContinuation    Opcode = 0x0
	OpcodeText            Opcode = 0x1
	OpcodeBinary          Opcode = 0x2
	OpcodeCloseConnection Opcode = 0x8
	OpcodePing            Opcode = 0x9
	OpcodePong            Opcode = 0xA
)

func (c Opcode) IsDataFrame() bool {
	return c <= OpcodeBinary
}

// WebSocket Event
// one of onclose and onerror will be called once during the connection's lifetime.
// 在连接的生命周期中，onclose和onerror中的一个有且只有一次被调用
type Event interface {
	OnOpen(socket *Conn)
	OnError(socket *Conn, err error)
	OnClose(socket *Conn, code uint16, reason []byte)
	OnPing(socket *Conn, payload []byte)
	OnPong(socket *Conn, payload []byte)
	OnMessage(socket *Conn, message *Message)
}

type BuiltinEventEngine struct{}

func (b BuiltinEventEngine) OnOpen(socket *Conn) {}

func (b BuiltinEventEngine) OnError(socket *Conn, err error) {}

func (b BuiltinEventEngine) OnClose(socket *Conn, code uint16, reason []byte) {}

func (b BuiltinEventEngine) OnPing(socket *Conn, payload []byte) {}

func (b BuiltinEventEngine) OnPong(socket *Conn, payload []byte) {}

func (b BuiltinEventEngine) OnMessage(socket *Conn, message *Message) {}
