package internal

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

func (p *DefaultPool) preAllocWorkers() {
	for i := 0; i < p.options.MaxWorkers; i++ {
		if atomic.AddInt32(&p.runningWorker, 1) <= int32(p.options.MaxWorkers) {
			p.wg.Add(1)
			// 分配时均匀分布在分片上
			go p.workerLoop(i)
		} else {
			atomic.AddInt32(&p.runningWorker, -1)
			break
		}
	}
}

func (p *DefaultPool) runTask(tw *taskWrapper) {
	ctx := tw.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	defer func() {
		if r := recover(); r != nil {
			buf := stackPool.Get().([]byte)
			n := runtime.Stack(buf, false)
			stackInfo := buf[:n]

			if p.options.PanicHandler != nil {
				p.options.PanicHandler(ctx, r, stackInfo)
			} else {
				fmt.Printf("workpool: internal panic recovered: %v\n", r)
			}
			stackPool.Put(buf)
		}
	}()

	atomic.AddInt32(&p.activeTasks, 1)
	defer atomic.AddInt32(&p.activeTasks, -1)

	if err := tw.task.Execute(ctx); err != nil {
		if p.options.FailureHandler != nil {
			p.options.FailureHandler(ctx, tw.task, err)
		}
	}
}

func (p *DefaultPool) workerLoop(shardIdx int) {
	defer p.wg.Done()
	defer atomic.AddInt32(&p.runningWorker, -1)

	// 绑定特定分片队列，极大降低锁竞争
	queue := p.taskQueues[uint64(shardIdx)&p.shardMask]

	idleTimer := time.NewTimer(p.options.ExpiryTime)
	defer idleTimer.Stop()

	for {
		select {
		case task, ok := <-queue:
			if !ok {
				return
			}
			p.runTask(task)

			// 清理并放回缓存
			task.task = nil
			task.ctx = nil
			p.workerCache.Put(task)

			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(p.options.ExpiryTime)
		case <-idleTimer.C:
			// 过期检查逻辑
			if atomic.LoadInt32(&p.runningWorker) > int32(runtime.NumCPU()) {
				return // 只有当 Worker 数超过 CPU 核数时才允许过期退出，保持基础吞吐
			}
			idleTimer.Reset(p.options.ExpiryTime)
		}
	}
}

func (p *DefaultPool) conditionallySpawnWorker() {
	curr := atomic.LoadInt32(&p.runningWorker)
	if curr < int32(p.options.MaxWorkers) {
		if n := atomic.AddInt32(&p.runningWorker, 1); n <= int32(p.options.MaxWorkers) {
			p.wg.Add(1)
			// 使用当前的 worker 计数作为索引，实现分片绑定
			go p.workerLoop(int(n - 1))
		} else {
			atomic.AddInt32(&p.runningWorker, -1)
		}
	}
}
