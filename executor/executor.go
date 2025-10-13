package executor

import (
	"context"
	"sync"
)

// Executor manages execution using different workflows such as worker pools.
// It supports different policies for handling work, such as waiting before erroring
// or rejecting immediately.
type Executor interface {
	// Execute will execute the received function and will return the
	// result of the executed function, or a reject error from the executor.
	Execute(ctx context.Context, f func() error) error
	WorkerPool
}

// WorkerPool maintains a worker pool that can dynamically increase and decrease the number of workers.
type WorkerPool interface {
	SetWorkerQuantity(quantity int)
	Shutdown()
}

// workerPool knows how to increase and decrease the current workers executing jobs.
// Its only objective is to set the desired number of concurrent execution flows.
type workerPool struct {
	workerStoppers []chan struct{}
	jobQueue       chan func()
	mu             sync.Mutex
	shutdownOnce   sync.Once
}

func newWorkerPool() workerPool {
	return workerPool{
		jobQueue: make(chan func()),
	}
}

// SetWorkerQuantity knows how to increase or decrease the worker pool.
func (w *workerPool) SetWorkerQuantity(quantity int) {
	if quantity < 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// If we don't need to increase or decrease the worker quantity then do nothing.
	if len(w.workerStoppers) == quantity {
		return
	}

	// If we have less workers than we need to add workers.
	if len(w.workerStoppers) < quantity {
		w.increaseWorkers(quantity - len(w.workerStoppers))
		return
	}

	// If we reached here then we need to reduce workers.
	w.decreaseWorkers(len(w.workerStoppers) - quantity)
}

func (w *workerPool) decreaseWorkers(workers int) {
	// Stop the not needed workers.
	toStop := w.workerStoppers[:workers]
	for _, stopC := range toStop {
		close(stopC)
	}

	// Set the new worker quantity.
	w.workerStoppers = w.workerStoppers[workers:]
}

func (w *workerPool) increaseWorkers(workers int) {
	for i := 0; i < workers; i++ {
		// Create a channel to stop the worker.
		stopC := make(chan struct{})
		go w.newWorker(stopC)
		w.workerStoppers = append(w.workerStoppers, stopC)
	}
}

// Shutdown gracefully stops all workers in the pool.
// This method is safe to call multiple times.
func (w *workerPool) Shutdown() {
	w.shutdownOnce.Do(func() {
		w.mu.Lock()
		defer w.mu.Unlock()

		// Close all worker stop channels
		for _, stopC := range w.workerStoppers {
			close(stopC)
		}

		// Clear the stoppers slice
		w.workerStoppers = nil
	})
}

func (w *workerPool) newWorker(stopC chan struct{}) {
	for {
		select {
		case <-stopC:
			return
		case f := <-w.jobQueue:
			f()
		}
	}
}
