package executor

import (
	"context"
	"time"
)

// LIFOConfig is the configuration for the LIFO executor.
type LIFOConfig struct {
	// MaxWaitTime is the maximum time a request will wait to execute before
	// being dropped and rejected.
	MaxWaitTime time.Duration

	// The LIFO queue uses a goroutine in background to execute the queue
	// jobs, in case it wants to be stopped a channel could be used to
	// stop the execution.
	StopChannel chan struct{}
}

func (c *LIFOConfig) defaults() {
	if c.MaxWaitTime == 0 {
		c.MaxWaitTime = 1 * time.Second
	}

	if c.StopChannel == nil {
		c.StopChannel = make(chan struct{})
	}
}

type lifo struct {
	cfg   LIFOConfig
	queue *dynamicQueue
	workerPool
}

// NewLIFO creates a LIFO (last-in-first-out) priority executor.
func NewLIFO(cfg LIFOConfig) Executor {
	cfg.defaults()

	l := &lifo{
		cfg:        cfg,
		workerPool: newWorkerPool(),
	}
	l.queue = newDynamicQueue(l.done(), enqueueAtEndPolicy, lifoDequeuePolicy)

	go func() {
		select {
		case <-cfg.StopChannel:
			l.Shutdown()
		case <-l.done():
		}
	}()
	go l.fromQueueToWorkerPool()

	return l
}

func (l *lifo) Execute(ctx context.Context, f func() error) error {
	// This channel will receive a signal when the job has been dequeued
	// to be processed.
	dequeuedJob := make(chan struct{})
	canceledJob := make(chan struct{})
	res := make(chan error, 1)

	timer := time.NewTimer(l.cfg.MaxWaitTime)
	defer timer.Stop()

	job := func() {
		// Send the signal the job has been dequeued.
		close(dequeuedJob)

		select {
		case <-canceledJob:
			return
		default:
		}

		res <- f()
	}

	select {
	case <-l.done():
		close(canceledJob)
		return ErrRejectedExecution
	case l.queue.InChannel() <- job:
	case <-timer.C:
		close(canceledJob)
		return ErrRejectedExecution
	case <-ctx.Done():
		close(canceledJob)
		return ctx.Err()
	}

	select {
	case <-l.done():
		close(canceledJob)
		return ErrRejectedExecution
	case <-timer.C:
		close(canceledJob)
		return ErrRejectedExecution
	case <-ctx.Done():
		close(canceledJob)
		return ctx.Err()
	case <-dequeuedJob:
		return <-res
	}
}

// fromQueueToWorkerPool will get from the queue in a loop the jobs to be
// executed by the worker pool.
func (l *lifo) fromQueueToWorkerPool() {
	for {
		select {
		case <-l.done():
			return
		case job := <-l.queue.OutChannel():
			// Send to execution worker.
			select {
			case <-l.done():
				return
			case l.workerPool.jobQueue <- job:
			}
		}
	}
}
