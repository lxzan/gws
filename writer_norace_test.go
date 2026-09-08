// 写路径零分配断言仅在非 -race 构建下运行：开启竞态检测时，runtime 会
// 刻意丢弃约 1/4 的 sync.Pool Put（用于暴露池化相关的数据竞争），导致池化
// 缓冲偶发走新建路径，分配数 >0，断言出现机制性抖动（并非真实的分配回归）。
// 该性质在常规构建下仍被完整断言。

//go:build !race

package gws

import (
	"net"
	"testing"

	"github.com/lxzan/gws/internal"
	"github.com/stretchr/testify/assert"
)

type infiniteWriter struct {
	net.TCPConn
}

func (c infiniteWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestConn_WriteMessageNoAllocations(t *testing.T) {
	var upgrader = NewUpgrader(&BuiltinEventHandler{}, nil)
	var conn = &Conn{
		conn:   &infiniteWriter{},
		config: upgrader.option.getConfig(),
	}
	var payload = internal.AlphabetNumeric.Generate(4096)
	var allocs = testing.AllocsPerRun(100, func() {
		_ = conn.WriteMessage(OpcodeText, payload)
	})
	assert.Equal(t, float64(0), allocs)
}
