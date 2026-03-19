package workpool

import (
	"workpool/internal"
	"workpool/pkg/strategy"
	"workpool/pkg/types"
)

type Option = types.Option
type Pool = types.Pool
type TaskFunc = types.TaskFunc

var (
	WithMaxWorkers   = types.WithMaxWorkers
	WithPanicHandler = types.WithPanicHandler
)

func NewPool(opts ...types.Option) (Pool, error) {
	options := types.DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	if options.MaxWorkers <= 0 {
		return nil, types.ErrInvalidPoolSize
	}

	if options.RejectPolicy == nil {
		options.RejectPolicy = strategy.AbortPolicy
	}

	return internal.NewDefaultPool(options), nil
}
