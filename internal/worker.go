package internal

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
	"workpool/pkg/types"
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
func (p *DefaultPool) runTask(task types.Task) {
	defer func() {
		if r := recover(); r != nil {
			if p.options.PanicHandler != nil {
				p.options.PanicHandler(r)
			} else {
				fmt.Printf("workpool: internal panic recovered: %v\n", r)
			}
		}
	}()

	atomic.AddInt32(&p.activeTasks, 1)        // 任务开始
	defer atomic.AddInt32(&p.activeTasks, -1) // 任务结束

	ctx := context.Background()
	_ = task.Execute(ctx)
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
