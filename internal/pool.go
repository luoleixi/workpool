package internal

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"workpool/pkg/types" // 替换为你的 go.mod 中的 module 名
)

const (
	STOPPED int32 = iota
	RUNNING
)

type DefaultPool struct {
	options   types.Options
	taskQueue chan types.Task // 任务中转站

	// 2. 状态控制（使用原子操作，尽量减少锁竞争）
	capacity      int32 // 最大容量 (MaxWorkers)
	runningWorker int32 // 当前活跃的 Goroutine 数量
	activeTasks   int32 // 真正正在执行任务的协程数 (新增)
	state         int32 // 池子状态：RUNNING / STOPPED

	// 3. Worker 管理与生命周期
	workerCache sync.Pool

	// 4. 优雅退出与同步
	wg   sync.WaitGroup // 等待所有 Worker 退出
	once sync.Once      // 确保 Release 只执行一次

	//lock         sync.Mutex
	//readyWorkers []*InternalWorker
}

func NewDefaultPool(opts types.Options) *DefaultPool {
	p := &DefaultPool{
		options:   opts,
		taskQueue: make(chan types.Task, opts.QueueSize),
		capacity:  int32(opts.MaxWorkers),
		state:     RUNNING,
	}
	if opts.PreAlloc {
		p.preAllocWorkers()
	}
	return p
}

func (p *DefaultPool) Submit(task types.Task) error {
	return p.SubmitWithContext(context.Background(), task)
}

func (p *DefaultPool) SubmitWithContext(ctx context.Context, task types.Task) error {
	if atomic.LoadInt32(&p.state) == STOPPED {
		return types.ErrPoolClosed
	}
	select {
	//调用方的 Context 超时了或被取消了
	case <-ctx.Done():
		return ctx.Err()
	//成功塞入队列
	case p.taskQueue <- task:
		p.conditionallySpawnWorker()
		return nil
	//队列瞬间满了。非阻塞尝试
	default:
		if p.options.RejectPolicy != nil {
			p.options.RejectPolicy(task, p)
		}
		return types.ErrPoolFull
	}
}

func (p *DefaultPool) Release() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.state, STOPPED)
		close(p.taskQueue)
		p.wg.Wait()
	})
}

func (p *DefaultPool) ReleaseWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		p.Release()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timed out")
	}
}

func (p *DefaultPool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, STOPPED, RUNNING) {
		p.once = sync.Once{}
		p.taskQueue = make(chan types.Task, p.options.QueueSize)
		if p.options.PreAlloc {
			p.preAllocWorkers()
		}
	}
}

func (p *DefaultPool) WaitingTasks() int {
	return len(p.taskQueue)
}

func (p *DefaultPool) RunningWorkers() int {
	return int(atomic.LoadInt32(&p.runningWorker))
}

func (p *DefaultPool) RunningTasks() int {
	return int(atomic.LoadInt32(&p.activeTasks))
}

func (p *DefaultPool) Cap() int {
	return p.options.MaxWorkers
}

func (p *DefaultPool) DiscardOldest() {
	select {
	case <-p.taskQueue:
		//成功弹出最老的任务并丢弃
	default:
		//如果队列为空，则什么都不做
	}
}
