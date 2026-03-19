package types

import (
	"context"
	"time"
)

type Pool interface {
	Submit(task Task) error
	SubmitWithContext(ctx context.Context, task Task) error
	Release()
	Reboot()
	WaitingTasks() int
	RunningTasks() int
	RunningWorkers() int
	Cap() int
	DiscardOldest()
	ReleaseWithTimeout(timeout time.Duration) error
}
