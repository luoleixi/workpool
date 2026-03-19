package types

import (
	"context"
	"runtime"
	"time"
)

const (
	defaultQueueSize = 100
	defaultExpiry    = 60 * time.Second
)

type RejectionHandler func(task Task, pool Pool)
type PanicHandler func(ctx context.Context, err interface{})
type Options struct {
	MaxWorkers   int
	QueueSize    int
	ExpiryTime   time.Duration
	PreAlloc     bool //是否预分配
	PanicHandler PanicHandler
	RejectPolicy RejectionHandler
}
type Option func(*Options)

func WithMaxWorkers(count int) Option {
	return func(o *Options) {
		o.MaxWorkers = count
	}
}

func WithPanicHandler(handler PanicHandler) Option {
	return func(o *Options) {
		o.PanicHandler = handler
	}
}

func DefaultOptions() Options {
	return Options{
		MaxWorkers:   runtime.NumCPU() * 2,
		QueueSize:    defaultQueueSize,
		ExpiryTime:   defaultExpiry,
		PreAlloc:     false, //默认不分配
		PanicHandler: nil,   //默认不处理
	}
}
