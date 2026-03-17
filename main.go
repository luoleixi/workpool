package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type task struct {
	f func() error
}

type Pool struct {
	tasks         chan task
	maxWorkers    int32
	activeWorkers int32
	wg            sync.WaitGroup
	idleTimeout   time.Duration
}

func NewPool(maxWorkers int32, idleTimeout time.Duration) *Pool {
	return &Pool{
		tasks:       make(chan task),
		maxWorkers:  maxWorkers,
		idleTimeout: idleTimeout,
	}
}

func (p *Pool) startWorker() {
	atomic.AddInt32(&p.activeWorkers, 1)
	p.wg.Add(1)
	go func() {
		defer func() {
			p.wg.Done()
			atomic.AddInt32(&p.activeWorkers, -1)
			fmt.Println("Worker 退出：闲置超时或池关闭")
		}()
		timer := time.NewTimer(p.idleTimeout)
		defer timer.Stop()
		for {
			select {
			case task, ok := <-p.tasks: //ok判断通道是否关闭
				if !ok {
					//关闭通道
					return
				}
				//停止定时器并重置，准备执行任务
				if !timer.Stop() {
					select {
					case <-timer.C:

					default:

					}
				}
				task.f()
				//任务执行完，重置定时器开始新一轮闲置倒计时
				timer.Reset(p.idleTimeout)
			case <-timer.C:
				//没拿到任务，触发超时退出
				return
			}
		}

	}()
}

func (p *Pool) Submit(f func() error) {
	// 如果当前没有工人或任务积压，且没到最大限制，就开个新工人
	if atomic.LoadInt32(&p.activeWorkers) < p.maxWorkers {
		p.startWorker()
	}
	p.tasks <- task{f: f}
}

func main() {

}
