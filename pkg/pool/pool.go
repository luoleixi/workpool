package pool

type WorkerPool interface {
	Submit()
}

type Task interface {
	Execute() error
}
