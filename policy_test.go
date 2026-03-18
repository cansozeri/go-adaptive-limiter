package limiter_test

import (
	"context"
	"errors"
	"testing"

	limiter "github.com/cansozeri/go-adaptive-limiter"
	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

func TestFailureOnExternalErrorPolicy(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want algorithm.Result
	}{
		{"nil error returns success", nil, algorithm.ResultSuccess},
		{"external error returns failure", errors.New("timeout"), algorithm.ResultFailure},
		{"rejected execution is ignored", executor.ErrRejectedExecution, algorithm.ResultIgnore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limiter.FailureOnExternalErrorPolicy(context.Background(), tt.err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoFailurePolicy(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want algorithm.Result
	}{
		{"nil error returns success", nil, algorithm.ResultSuccess},
		{"any error is ignored", errors.New("timeout"), algorithm.ResultIgnore},
		{"rejected execution is ignored", executor.ErrRejectedExecution, algorithm.ResultIgnore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limiter.NoFailurePolicy(context.Background(), tt.err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFailureOnRejectedPolicy(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want algorithm.Result
	}{
		{"nil error returns success", nil, algorithm.ResultSuccess},
		{"external error is ignored", errors.New("timeout"), algorithm.ResultIgnore},
		{"rejected execution returns failure", executor.ErrRejectedExecution, algorithm.ResultFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limiter.FailureOnRejectedPolicy(context.Background(), tt.err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFailureOnExternalErrorPolicy_WrappedError(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), executor.ErrRejectedExecution)

	got := limiter.FailureOnExternalErrorPolicy(context.Background(), wrapped)
	if got != algorithm.ResultIgnore {
		t.Errorf("wrapped rejected error should be ignored, got %v", got)
	}
}

func TestFailureOnRejectedPolicy_WrappedError(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), executor.ErrRejectedExecution)

	got := limiter.FailureOnRejectedPolicy(context.Background(), wrapped)
	if got != algorithm.ResultFailure {
		t.Errorf("wrapped rejected error should be failure, got %v", got)
	}
}
