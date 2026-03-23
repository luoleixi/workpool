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
	options    types.Options
	taskQueues []chan *taskWrapper
	shardMask  uint64 // 用于快速取模 (需为 2 的幂)
	queueIdx   uint64 // 原子操作，用于轮询分发任务

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

type taskWrapper struct {
	task types.Task
	ctx  context.Context
}

func (p *DefaultPool) initPools() {
	p.workerCache = sync.Pool{
		New: func() interface{} {
			return &taskWrapper{}
		},
	}
}

var stackPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}

func NewDefaultPool(opts types.Options) *DefaultPool {
	shardCount := 16
	queues := make([]chan *taskWrapper, shardCount)
	for i := 0; i < shardCount; i++ {
		// 每个分片的容量可以平分总 QueueSize
		queues[i] = make(chan *taskWrapper, opts.QueueSize/shardCount+1)
	}
	p := &DefaultPool{
		options:    opts,
		taskQueues: queues,
		shardMask:  uint64(shardCount - 1),
		capacity:   int32(opts.MaxWorkers),
		state:      RUNNING,
	}

	p.workerCache.New = func() interface{} {
		return &taskWrapper{}
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

	tw := p.workerCache.Get().(*taskWrapper)
	tw.task = task
	tw.ctx = ctx

	// 快速轮询分片
	idx := atomic.AddUint64(&p.queueIdx, 1) & p.shardMask

	select {
	case <-ctx.Done():
		p.workerCache.Put(tw)
		return ctx.Err()
	case p.taskQueues[idx] <- tw:
		p.conditionallySpawnWorker()
		return nil
	default:
		// 备选：如果选中的分片满了，可以尝试一次临近分片减少拒绝率
		nextIdx := (idx + 1) & p.shardMask
		select {
		case p.taskQueues[nextIdx] <- tw:
			p.conditionallySpawnWorker()
			return nil
		default:
			p.workerCache.Put(tw)
			if p.options.RejectPolicy != nil {
				p.options.RejectPolicy(task, p)
			}
			return types.ErrPoolFull
		}
	}
}

func (p *DefaultPool) Release() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.state, STOPPED)
		// 修改：关闭所有分片队列
		for _, q := range p.taskQueues {
			close(q)
		}
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
		// 重新初始化所有队列
		for i := range p.taskQueues {
			p.taskQueues[i] = make(chan *taskWrapper, p.options.QueueSize/len(p.taskQueues)+1)
		}
		if p.options.PreAlloc {
			p.preAllocWorkers()
		}
	}
}

func (p *DefaultPool) WaitingTasks() int {
	var count int
	for _, q := range p.taskQueues {
		count += len(q)
	}
	return count
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
	// 随机丢弃一个队列的最老任务，平衡性能
	idx := atomic.LoadUint64(&p.queueIdx) & p.shardMask
	select {
	case <-p.taskQueues[idx]:
	default:
	}
}
