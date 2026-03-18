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
	return p.SubmitWithContext(context.Background(), task)
}

func (p *DefaultPool) conditionallySpawnWorker() {
	run := atomic.LoadInt32(&p.runningWorker)
	if run >= p.capacity {
		return
	}

	//当前没有协程在运行 或 队列中有积压任务且有扩容空间，触发扩容
	if run == 0 || len(p.taskQueue) > 0 {
		if atomic.CompareAndSwapInt32(&p.runningWorker, run, run+1) {
			p.wg.Add(1)
			go p.workerLoop()
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
			//动态缩容，如果超时无任务，且队列为空。销毁协程
			if len(p.taskQueue) == 0 {
				return
			}
		}
	}

}

func (p *DefaultPool) SubmitWithContext(ctx context.Context, task Task) error {
	if atomic.LoadInt32(&p.state) == STOPPED {
		return ErrPoolClosed
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
		return ErrPoolFull
	}
}

func (p *DefaultPool) Release() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.state, STOPPED)
		close(p.taskQueue)
		p.wg.Wait()
	})
}

func (p *DefaultPool) Reboot() {
	if atomic.CompareAndSwapInt32(&p.state, STOPPED, RUNNING) {
		p.once = sync.Once{}
		p.taskQueue = make(chan Task, p.options.QueueSize)
		if p.options.PreAlloc {
			p.preAllocWorkers()
		}
	}
}

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
		p.preAllocWorkers()
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
