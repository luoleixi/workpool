package workpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrInvalidPoolSize 当初始化时传入的 MaxWorkers <= 0 时触发
	ErrInvalidPoolSize = errors.New("workpool: invalid pool size, must be greater than 0")

	// ErrPoolClosed 当向一个已经 Release 的池子提交任务时触发
	ErrPoolClosed = errors.New("workpool: the pool has been closed")

	// ErrPoolFull 当任务队列已满且拒绝策略为同步拒绝时触发
	ErrPoolFull = errors.New("workpool: the task queue is full")

	// ErrTimeout 当 SubmitWithTimeout 在规定时间内未将任务送入队列时触发
	ErrTimeout = errors.New("workpool: submit task timeout")
)

type Pool interface {
	Submit(task Task) error
	SubmitWithContext(ctx context.Context, task Task) error
	Release()
	Reboot()
	WaitingTasks() int
	RunningTasks() int
	Cap() int
}

type DefaultPool struct {
	options   Options
	taskQueue chan Task // 任务中转站

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

func (p *DefaultPool) Submit(task Task) error {
	if atomic.LoadInt32(&p.state) == STOPPED {
		return ErrPoolClosed
	}
	select {
	case p.taskQueue <- task:
		p.conditionallySpawnWorker()
		return nil
	default:
		if p.options.RejectPolicy != nil {
			p.options.RejectPolicy(task, p)
		}
		return ErrPoolFull
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

func (p *DefaultPool) workerLoop() {
	defer p.wg.Done()
	defer atomic.AddInt32(&p.runningWorker, -1)

	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			//执行任务并处理 Panic
			p.runTask(task)
		case <-time.After(p.options.ExpiryTime):
			//空间超时自动回收协程
			if len(p.taskQueue) == 0 {
				return
			}
		}
	}

}

func (p *DefaultPool) SubmitWithContext(ctx context.Context, task Task) error {
	//TODO implement me
	panic("implement me")
}

func (p *DefaultPool) Release() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.state, STOPPED)
		close(p.taskQueue)
		p.wg.Wait()
	})
}

func (p *DefaultPool) Reboot() {
	//TODO implement me
	panic("implement me")
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

const (
	STOPPED int32 = iota
	RUNNING
)

func NewPool(opts ...Option) (Pool, error) {
	options := DefaultOptions()

	for _, opt := range opts {
		opt(&options)
	}

	if options.MaxWorkers <= 0 {
		return nil, ErrInvalidPoolSize
	}

	p := &DefaultPool{
		options:   options,
		taskQueue: make(chan Task, options.QueueSize),
		capacity:  int32(options.MaxWorkers),
		state:     RUNNING,
	}

	//预分配逻辑
	if options.PreAlloc {
		for i := 0; i < options.MaxWorkers; i++ {

		}
	}
	return p, nil
}

func (p *DefaultPool) runTask(task Task) {
	defer func() {
		if r := recover(); r != nil {
			if p.options.PanicHandler != nil {
				p.options.PanicHandler(r)
			} else {
				fmt.Printf("workpool: internal panic recovered: %v\n", r)
			}
		}
	}()
	ctx := context.Background()
	_ = task.Execute(ctx)
}
