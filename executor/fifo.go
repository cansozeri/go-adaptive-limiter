package executor

import (
	"context"
	"time"
)

// FIFOConfig is the configuration for the FIFO executor.
type FIFOConfig struct {
	// MaxWaitTime is the maximum time a request will wait to execute before
	// being dropped and rejected.
	MaxWaitTime time.Duration
}

func (c *FIFOConfig) defaults() {
	if c.MaxWaitTime == 0 {
		c.MaxWaitTime = 1 * time.Second
	}
}

// NewFIFO creates a FIFO executor that executes requests when workers are available.
// If no workers are available, requests are queued with FIFO priority until a worker is free
// or the timeout is reached, in which case the execution is rejected.
//
// The FIFO queue leverages Go's channel implementation which guarantees first-in-first-out
// ordering for blocked sends.
func NewFIFO(cfg FIFOConfig) Executor {
	cfg.defaults()

	return &fifo{
		workerPool: newWorkerPool(),
		cfg:        cfg,
	}
}

type fifo struct {
	cfg FIFOConfig
	workerPool
}

// Execute satisfies Executor interface.
func (f *fifo) Execute(ctx context.Context, fn func() error) error {
	result := make(chan error, 1)
	job := func() {
		result <- fn()
	}

	timer := time.NewTimer(f.cfg.MaxWaitTime)
	defer timer.Stop()

	select {
	case <-f.done():
		return ErrRejectedExecution
	case f.jobQueue <- job:
		return <-result
	case <-timer.C:
		return ErrRejectedExecution
	case <-ctx.Done():
		return ctx.Err()
	}
}
