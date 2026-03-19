package types

import "context"

type Task interface {
	Execute(ctx context.Context) error
}

type TaskFunc func(ctx context.Context) error

func (f TaskFunc) Execute(ctx context.Context) error {
	return f(ctx)
}
