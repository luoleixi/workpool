package internal

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

func (p *DefaultPool) preAllocWorkers() {
	for i := 0; i < p.options.MaxWorkers; i++ {
		if atomic.AddInt32(&p.runningWorker, 1) <= int32(p.options.MaxWorkers) {
			p.wg.Add(1)
			go p.workerLoop()
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
		if r := recover(); r != nil { //捕获异常
			if p.options.PanicHandler != nil {
				//将异常转发给自定义处理器
				p.options.PanicHandler(ctx, r)
			} else {
				//确保 Panic 不会消失
				fmt.Printf("workpool: internal panic recovered: %v\n", r)
			}
		}
	}()

	atomic.AddInt32(&p.activeTasks, 1)        // 任务开始
	defer atomic.AddInt32(&p.activeTasks, -1) // 任务结束

	if err := tw.task.Execute(ctx); err != nil {
		if p.options.FailureHandler != nil {
			p.options.FailureHandler(ctx, tw.task, err)
		}
	}
}

func (p *DefaultPool) workerLoop() {
	defer p.wg.Done()
	defer atomic.AddInt32(&p.runningWorker, -1)

	idleTimer := time.NewTimer(p.options.ExpiryTime)
	defer idleTimer.Stop()

	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			p.runTask(task)

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
			if len(p.taskQueue) == 0 {
				return
			}
			idleTimer.Reset(p.options.ExpiryTime)
		}
	}
}

func (p *DefaultPool) conditionallySpawnWorker() {
	if atomic.LoadInt32(&p.runningWorker) < int32(p.options.MaxWorkers) {
		if atomic.AddInt32(&p.runningWorker, 1) <= int32(p.options.MaxWorkers) {
			p.wg.Add(1)
			go p.workerLoop()
		} else {
			// 如果并发争抢导致超过了 Max，再减回来
			atomic.AddInt32(&p.runningWorker, -1)
		}
	}
}
