package limiter_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cansozeri/go-adaptive-limiter"
	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
	"github.com/stretchr/testify/assert"
)

func TestLimiter_Execute(t *testing.T) {
	a := assert.New(t)

	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(10)),
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{
			MaxWaitTime: 1 * time.Second,
		})),
	)

	err := lim.Execute(context.Background(), func() error {
		return nil
	})

	a.NoError(err)
}

func TestLimiter_ExecuteWithResult(t *testing.T) {
	a := assert.New(t)

	lim := limiter.New()

	result, err := limiter.ExecuteWithResult(lim, context.Background(), func() (string, error) {
		return "success", nil
	})

	a.NoError(err)
	a.Equal("success", result)
}

func TestLimiter_Stats(t *testing.T) {
	a := assert.New(t)

	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(5)),
	)

	stats := lim.Stats()
	a.Equal(5, stats.CurrentLimit)
	a.Equal(0, stats.InFlight)
	a.Equal(0, stats.Executing)
}

func TestLimiter_ConcurrentExecutions(t *testing.T) {
	a := assert.New(t)

	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(10)),
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{
			MaxWaitTime: 500 * time.Millisecond,
		})),
	)

	var success atomic.Int32
	done := make(chan bool)

	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- true }()

			err := lim.Execute(context.Background(), func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})

			if err == nil {
				success.Add(1)
			}
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	a.Greater(success.Load(), int32(0), "some executions should succeed")
}

func TestLimiter_ContextCancellation(t *testing.T) {
	a := assert.New(t)

	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(0)), // No workers
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{
			MaxWaitTime: 5 * time.Second,
		})),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := lim.Execute(ctx, func() error {
		return nil
	})

	elapsed := time.Since(start)

	a.Error(err)
	a.Less(elapsed, 200*time.Millisecond, "should cancel quickly")
}

func TestLimiter_Shutdown(t *testing.T) {
	a := assert.New(t)

	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(1)),
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{
			MaxWaitTime: time.Second,
		})),
	)

	started := make(chan struct{})
	release := make(chan struct{})
	execDone := make(chan error, 1)

	go func() {
		execDone <- lim.Execute(context.Background(), func() error {
			close(started)
			<-release
			return nil
		})
	}()

	<-started

	shutdownDone := make(chan struct{})
	go func() {
		lim.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned before the in-flight request completed")
	case <-time.After(50 * time.Millisecond):
	}

	start := time.Now()
	err := lim.Execute(context.Background(), func() error {
		return nil
	})
	elapsed := time.Since(start)

	a.ErrorIs(err, limiter.ErrRejectedExecution)
	a.Less(elapsed, 100*time.Millisecond, "shutdown should reject new work immediately")

	close(release)

	select {
	case err := <-execDone:
		a.NoError(err)
	case <-time.After(time.Second):
		t.Fatal("in-flight request did not complete")
	}

	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not wait for in-flight work to finish")
	}

	// Shutdown should remain safe to call multiple times.
	lim.Shutdown()
}

func TestLimiter_ShutdownReturnsRejectedExecution(t *testing.T) {
	lim := limiter.New()
	lim.Shutdown()

	start := time.Now()
	err := lim.Execute(context.Background(), func() error {
		return nil
	})

	if !errors.Is(err, limiter.ErrRejectedExecution) {
		t.Fatalf("expected rejected execution after shutdown, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected immediate rejection after shutdown, took %v", elapsed)
	}
}
