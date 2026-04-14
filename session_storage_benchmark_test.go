package gws

import (
	"github.com/lxzan/gws/internal"
	"strconv"
	"testing"
)

// BenchmarkConcurrentMap 性能测试, 读写比例可配置.
// 使用方式示例:
//
//	go test -run=^$ -bench=BenchmarkConcurrentMap -benchtime=3s \
//	  -concurrentmap_read_ratio=80 -benchmem
func BenchmarkConcurrentMap(b *testing.B) {
	// 为了便于对比，不同子用例只调整读写比例
	ratios := []int{0, 50, 80, 90, 100}
	for _, r := range ratios {
		ratio := r
		name := "read_" + strconv.Itoa(ratio)
		b.Run(name, func(b *testing.B) {
			benchmarkConcurrentMapWithRatio(b, ratio)
		})
	}
}

func benchmarkConcurrentMapWithRatio(b *testing.B, readRatio int) {
	if readRatio < 0 {
		readRatio = 0
	}
	if readRatio > 100 {
		readRatio = 100
	}

	const (
		initPerMap = 1024 * 64 // 预填充的 key 数量
	)

	// 预先创建并填充一份 map, 避免把构建开销算进基准。
	// key 使用随机字符串, 更贴近真实场景。
	keys := make([]string, initPerMap)
	for i := uint64(0); i < initPerMap; i++ {
		keys[i] = string(internal.AlphabetNumeric.Generate(16))
	}
	m := NewConcurrentMap[string, uint64]()
	for i := uint64(0); i < initPerMap; i++ {
		m.Store(keys[i], i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var (
			opIndex uint64
		)
		for pb.Next() {
			opIndex++
			idx := opIndex % initPerMap
			key := keys[idx]

			// 简单的基于计数的读写选择, 避免在热路径中调用 rand。
			if int(opIndex%100) < readRatio {
				m.Load(key)
			} else {
				// 写操作使用 Store 即可, 覆盖写能模拟典型的更新场景
				m.Store(key, opIndex)
				// 这里不执行 Delete, 避免对 key 空洞的影响导致额外分支
			}
		}
	})
}
