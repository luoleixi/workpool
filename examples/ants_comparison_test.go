package main

import (
	"context"
	"github.com/panjf2000/ants/v2"
	"sync"
	"testing"
	"workpool"
	"workpool/pkg/types"
)

const (
	RunTimes    = 1000000 // 100万个任务
	WorkPayload = 100     // 模拟任务负载量（循环次数）
)

// --- 统一负载函数 ---
// 模拟真实的计算消耗，避免任务瞬间结束导致的调度器“空转”
func doWork() {
	sum := 0
	for i := 0; i < WorkPayload; i++ {
		sum += i
	}
	_ = sum
}

// --- 1. 原生 Goroutine 测试 ---
func BenchmarkNativeGoroutine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			go func() {
				doWork()
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

// --- 2. Ants PoolWithFunc 测试 ---
func BenchmarkAntsPoolWithFunc(b *testing.B) {
	// 定义 ants 的任务处理器
	p, _ := ants.NewPoolWithFunc(50000, func(i interface{}) {
		doWork()
		i.(*sync.WaitGroup).Done()
	})
	defer p.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			_ = p.Invoke(&wg)
		}
		wg.Wait()
	}
}

// --- 3. 你的 WorkPool 测试 ---
func BenchmarkWorkPoolStatic(b *testing.B) {
	p, _ := workpool.NewPool(
		workpool.WithMaxWorkers(50000), // 调高容量以匹配 ants
	)
	defer p.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(RunTimes)

		// 预创建一个静态 TaskFunc，彻底消除闭包分配
		task := types.TaskFunc(func(ctx context.Context) error {
			doWork()
			wg.Done()
			return nil
		})

		for j := 0; j < RunTimes; j++ {
			// 如果 Submit 报错（如队列满），由当前协程同步执行，保证 WaitGroup 计数准确
			if err := p.Submit(task); err != nil {
				_ = task.Execute(context.Background())
			}
		}
		wg.Wait()
	}
}
