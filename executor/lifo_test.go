package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLIFO_Execute_Success(t *testing.T) {
	stopChan := make(chan struct{})
	defer close(stopChan)

	exec := NewLIFO(LIFOConfig{
		MaxWaitTime: 1 * time.Second,
		StopChannel: stopChan,
	})
	exec.SetWorkerQuantity(2)

	executed := false
	err := exec.Execute(context.Background(), func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !executed {
		t.Error("function was not executed")
	}
}

func TestLIFO_Execute_Timeout(t *testing.T) {
	stopChan := make(chan struct{})
	defer close(stopChan)

	exec := NewLIFO(LIFOConfig{
		MaxWaitTime: 50 * time.Millisecond,
		StopChannel: stopChan,
	})
	exec.SetWorkerQuantity(0)

	err := exec.Execute(context.Background(), func() error {
		return nil
	})

	if !errors.Is(err, ErrRejectedExecution) {
		t.Errorf("expected ErrRejectedExecution, got %v", err)
	}
}

func TestLIFO_Execute_ContextCancellation(t *testing.T) {
	stopChan := make(chan struct{})
	defer close(stopChan)

	exec := NewLIFO(LIFOConfig{
		MaxWaitTime: 5 * time.Second,
		StopChannel: stopChan,
	})
	exec.SetWorkerQuantity(0)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := exec.Execute(ctx, func() error {
		return nil
	})

	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Errorf("took too long: %v", elapsed)
	}

	if err != context.DeadlineExceeded {
		t.Errorf("expected context deadline exceeded, got %v", err)
	}
}

func TestLIFO_ConcurrentExecutions(t *testing.T) {
	stopChan := make(chan struct{})
	defer close(stopChan)

	exec := NewLIFO(LIFOConfig{
		MaxWaitTime: 1 * time.Second,
		StopChannel: stopChan,
	})
	exec.SetWorkerQuantity(5)

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			err := exec.Execute(context.Background(), func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			if err == nil {
				done <- true
			}
		}()
	}

	success := 0
	timeout := time.After(2 * time.Second)
	for success < 20 {
		select {
		case <-done:
			success++
		case <-timeout:
			t.Fatalf("timeout waiting for executions, only %d succeeded", success)
		}
	}
}

func TestLIFO_FunctionError(t *testing.T) {
	stopChan := make(chan struct{})
	defer close(stopChan)

	exec := NewLIFO(LIFOConfig{
		MaxWaitTime: 1 * time.Second,
		StopChannel: stopChan,
	})
	exec.SetWorkerQuantity(1)

	expectedErr := errors.New("test error")
	err := exec.Execute(context.Background(), func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestLIFO_DefaultConfig(t *testing.T) {
	exec := NewLIFO(LIFOConfig{})
	lifo := exec.(*lifo)

	if lifo.cfg.MaxWaitTime != 1*time.Second {
		t.Errorf("expected default MaxWaitTime 1s, got %v", lifo.cfg.MaxWaitTime)
	}
	if lifo.cfg.StopChannel == nil {
		t.Error("expected default StopChannel to be initialized")
	}
}

func TestLIFO_Shutdown(t *testing.T) {
	stopChan := make(chan struct{})

	exec := NewLIFO(LIFOConfig{
		MaxWaitTime: 1 * time.Second,
		StopChannel: stopChan,
	})
	exec.SetWorkerQuantity(2)

	exec.Shutdown()

	// Multiple shutdowns should not panic
	exec.Shutdown()
}
