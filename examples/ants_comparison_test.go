package main

import (
	"context"
	"github.com/panjf2000/ants/v2"
	"sync"
	"testing"
	"workpool"
)

const (
	RunTimes    = 1000000
	WorkPayload = 100
)

func doWork() {
	sum := 0
	for i := 0; i < WorkPayload; i++ {
		sum += i
	}
	_ = sum
}

// 1. 原生 Goroutine
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

func BenchmarkWorkPoolGeneric(b *testing.B) {
	// 1. 定义执行器：任务结束后负责将参数归还池子
	exec := func(ctx context.Context, data *MyContext) error {
		doWork()
		data.wg.Done()
		data.wg = nil // 必须置空，防止内存泄漏
		contextPool.Put(data)
		return nil
	}

	p, _ := workpool.NewGenericPool[*MyContext](exec, workpool.WithMaxWorkers[*MyContext](50000))
	defer p.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			// 2. 从池子里拿 data，而不是 &MyContext{}
			data := contextPool.Get().(*MyContext)
			data.wg = &wg

			if err := p.Submit(data); err != nil {
				// 3. 失败也要手动执行并回收
				data.wg.Done()
				contextPool.Put(data)
			}
		}
		wg.Wait()
	}
}

// 2. Ants 测试
func BenchmarkAntsPoolWithFunc(b *testing.B) {
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

// 3. 泛型 WorkPool 极致性能版
type MyContext struct {
	wg *sync.WaitGroup
}

// 专门为任务参数准备的对象池
var contextPool = sync.Pool{
	New: func() interface{} { return &MyContext{} },
}
