package workpool

import (
	"context"
	"errors"
	"sync"
	"workpool/pkg/pool"
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
	taskQueue chan Task

	capacity      int32 //最大容量
	runningWorker int32 //当前运行中的Worker数量
	state         int32 //池子状态

	lock    sync.Locker        //保护内部Worker 列表
	workers []*pool.WorkerPool //内部Worker 列表

	cond *sync.Cond
}

func (p *DefaultPool) Submit(task Task) error {
	return nil
}

func (p *DefaultPool) conditionallySpawnWorker() {

}

func (p *DefaultPool) SubmitWithContext(ctx context.Context, task Task) error {
	//TODO implement me
	panic("implement me")
}

func (p *DefaultPool) Release() {
	//TODO implement me
	panic("implement me")
}

func (p *DefaultPool) Reboot() {
	//TODO implement me
	panic("implement me")
}

func (p *DefaultPool) WaitingTasks() int {
	//TODO implement me
	panic("implement me")
}

func (p *DefaultPool) RunningTasks() int {
	//TODO implement me
	panic("implement me")
}

func (p *DefaultPool) Cap() int {
	//TODO implement me
	panic("implement me")
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
		lock:      &sync.Mutex{},
	}

	//预分配逻辑
	if options.PreAlloc {
		for i := 0; i < options.MaxWorkers; i++ {

		}
	}
	return p, nil
}
