package types

import "context"

type Pool interface {
	Submit(task Task) error
	SubmitWithContext(ctx context.Context, task Task) error
	Release()
	Reboot()
	WaitingTasks() int
	RunningTasks() int
	RunningWorkers() int
	Cap() int
}
