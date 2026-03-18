// Package limiter provides adaptive concurrency limiting based on TCP congestion control.
//
// Inspired by Netflix's concurrency-limits library and TCP algorithms (AIMD, Vegas),
// this package dynamically adjusts concurrency limits using Little's Law.
//
// Basic usage:
//
//	lim := limiter.New(
//	    limiter.WithAlgorithm(algorithm.NewAIMD(algorithm.AIMDConfig{
//	        MinimumLimit: 10,
//	        RTTTimeout: 100 * time.Millisecond,
//	    })),
//	)
//
//	err := lim.Execute(ctx, func() error {
//	    return doWork()
//	})
package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

// Re-export common errors for convenience
var ErrRejectedExecution = executor.ErrRejectedExecution

// Limiter manages concurrency limits using adaptive algorithms.
type Limiter struct {
	executor executor.Executor
	alg      algorithm.Limiter
	policy   ResultPolicy

	inFlights atomicCounter
	executing atomicCounter

	shutdownMu      sync.Mutex
	shutdownCond    *sync.Cond
	shuttingDown    bool
	activeExecution int
}

// ResultPolicy determines how execution results should be categorized for the algorithm.
type ResultPolicy func(ctx context.Context, err error) algorithm.Result

// New creates a new adaptive limiter with the given options.
func New(opts ...Option) *Limiter {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	lim := &Limiter{
		executor: cfg.executor,
		alg:      cfg.algorithm,
		policy:   cfg.policy,
	}
	lim.shutdownCond = sync.NewCond(&lim.shutdownMu)

	// Set initial limit
	lim.executor.SetWorkerQuantity(lim.alg.GetLimit())

	return lim
}

// Execute runs the given function with concurrency limiting.
// Returns an error if execution is rejected or if the function returns an error.
func (l *Limiter) Execute(ctx context.Context, fn func() error) error {
	l.shutdownMu.Lock()
	if l.shuttingDown {
		l.shutdownMu.Unlock()
		return ErrRejectedExecution
	}
	l.activeExecution++
	l.shutdownMu.Unlock()

	start := time.Now()
	var queuedDuration time.Duration
	var err error

	l.inFlights.Inc()
	defer func() {
		currentFlights := l.inFlights.Dec()
		l.shutdownMu.Lock()
		l.activeExecution--
		skipAdaptation := l.shuttingDown
		if l.activeExecution == 0 {
			l.shutdownCond.Broadcast()
		}
		l.shutdownMu.Unlock()

		// Measure and adapt limit based on the execution result
		if l.policy != nil && !skipAdaptation {
			result := l.policy(ctx, err)
			if result != algorithm.ResultIgnore {
				newLimit := l.alg.MeasureSample(start, queuedDuration, currentFlights, result)
				l.executor.SetWorkerQuantity(newLimit)
			}
		}
	}()

	// Execute with the executor
	err = l.executor.Execute(ctx, func() error {
		queuedDuration = time.Since(start)
		l.executing.Inc()
		defer l.executing.Dec()

		return fn()
	})

	return err
}

// ExecuteWithResult runs the given function and returns both result and error.
func ExecuteWithResult[T any](l *Limiter, ctx context.Context, fn func() (T, error)) (T, error) {
	var result T
	var fnErr error

	err := l.Execute(ctx, func() error {
		result, fnErr = fn()
		return fnErr
	})
	if err != nil {
		var zero T
		return zero, err
	}

	return result, fnErr
}

// Stats returns current limiter statistics.
// InFlight counts all admitted requests, including both queued and executing work.
func (l *Limiter) Stats() Stats {
	return Stats{
		CurrentLimit: l.alg.GetLimit(),
		InFlight:     int(l.inFlights.c.Load()),
		Executing:    int(l.executing.c.Load()),
	}
}

// Stats holds limiter statistics.
type Stats struct {
	CurrentLimit int
	InFlight     int
	Executing    int
}

// Shutdown stops accepting new work, waits for admitted work to finish, and then
// shuts down the underlying executor.
func (l *Limiter) Shutdown() {
	l.shutdownMu.Lock()
	l.shuttingDown = true
	for l.activeExecution > 0 {
		l.shutdownCond.Wait()
	}
	l.shutdownMu.Unlock()

	if l.executor != nil {
		l.executor.Shutdown()
	}
}

type atomicCounter struct {
	c atomic.Int64
}

func (a *atomicCounter) Inc() int {
	return int(a.c.Add(1))
}

func (a *atomicCounter) Dec() int {
	return int(a.c.Add(-1))
}
