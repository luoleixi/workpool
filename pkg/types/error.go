package types

import "errors"

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
