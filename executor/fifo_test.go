package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFIFO_Execute_Success(t *testing.T) {
	exec := NewFIFO(FIFOConfig{
		MaxWaitTime: 1 * time.Second,
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

func TestFIFO_Execute_Timeout(t *testing.T) {
	exec := NewFIFO(FIFOConfig{
		MaxWaitTime: 50 * time.Millisecond,
	})
	exec.SetWorkerQuantity(0)

	err := exec.Execute(context.Background(), func() error {
		return nil
	})

	if !errors.Is(err, ErrRejectedExecution) {
		t.Errorf("expected ErrRejectedExecution, got %v", err)
	}
}

func TestFIFO_Execute_ContextCancellation(t *testing.T) {
	exec := NewFIFO(FIFOConfig{
		MaxWaitTime: 5 * time.Second,
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

func TestFIFO_ConcurrentExecutions(t *testing.T) {
	exec := NewFIFO(FIFOConfig{
		MaxWaitTime: 1 * time.Second,
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

func TestFIFO_FunctionError(t *testing.T) {
	exec := NewFIFO(FIFOConfig{
		MaxWaitTime: 1 * time.Second,
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

func TestFIFO_DefaultConfig(t *testing.T) {
	exec := NewFIFO(FIFOConfig{})
	fifo := exec.(*fifo)

	if fifo.cfg.MaxWaitTime != 1*time.Second {
		t.Errorf("expected default MaxWaitTime 1s, got %v", fifo.cfg.MaxWaitTime)
	}
}
