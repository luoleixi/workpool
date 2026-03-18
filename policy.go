package workpool

type RejectionHandler func(task Task, pool Pool)

var (
	// AbortPolicy 默认策略：直接抛出错误（由 Submit 返回 ErrPoolFull）
	AborPolicy RejectionHandler = func(task Task, pool Pool) {

	}
	// DiscardPolicy 丢弃策略：静默丢弃最新提交的任务，不报任何错误
	DiscardPolicy RejectionHandler = func(task Task, pool Pool) {

	}
	// CallerRunsPolicy 调用者运行策略：由提交任务的当前 Goroutine 直接执行该任务
	CallerRunsPolicy RejectionHandler = func(task Task, pool Pool) {

	}

	// DiscardOldestPolicy 丢弃最老策略：丢弃队列中最前面的任务，腾出空间给新任务
	DiscardOldestPolicy RejectionHandler = func(task Task, pool Pool) {

	}
)
