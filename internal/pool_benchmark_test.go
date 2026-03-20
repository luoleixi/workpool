package internal

import (
	"context"
	"sync"
	"testing"
	"time"
	"workpool/pkg/types"
)

// 调整后的任务：1ms 负载，方便测试在 1s 内看到结果
// 如果设为 10s，测试可能需要跑几个小时才能结束
const (
	TaskLoad    = 1 * time.Millisecond
	Concurrency = 1000000 // 一百万并发
)

// 场景 1: 原生 Goroutine (极高概率 OOM)
func BenchmarkNativeGoroutine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		b.StartTimer()
		for j := 0; j < Concurrency; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(TaskLoad)
			}()
		}
		wg.Wait()
	}
}

// 场景 2: 使用 WorkPool (稳定运行)
func BenchmarkWorkPool(b *testing.B) {
	opts := types.DefaultOptions()
	opts.MaxWorkers = 5000       // 提高 Worker 数提升吞吐
	opts.QueueSize = Concurrency // 队列撑满一百万
	p := NewDefaultPool(opts)
	defer p.Release()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		b.StartTimer()
		for j := 0; j < Concurrency; j++ {
			wg.Add(1)
			// 使用 TaskFunc 减少结构体创建开销
			err := p.Submit(types.TaskFunc(func(ctx context.Context) error {
				defer wg.Done()
				time.Sleep(TaskLoad)
				return nil
			}))

			// 如果队列满了（虽然我们设置了很大，但预防万一）
			if err != nil {
				wg.Done()
			}
		}
		wg.Wait()
	}
}
