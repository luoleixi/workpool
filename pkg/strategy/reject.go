package strategy

import (
	"context"
	"workpool/pkg/types"
)

var (
	AbortPolicy types.RejectionHandler = func(task types.Task, pool types.Pool) {
		// SubmitWithContext 会处理 ErrPoolFull
	}
	CallerRunsPolicy types.RejectionHandler = func(task types.Task, pool types.Pool) {
		_ = task.Execute(context.Background())
	}

	DiscardPolicy types.RejectionHandler = func(task types.Task, pool types.Pool) {
		// 静默丢弃，什么都不做
	}
)
