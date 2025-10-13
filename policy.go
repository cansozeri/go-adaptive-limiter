package limiter

import (
	"context"

	goErrors "errors"

	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

// ResultPolicy is defined in limiter.go

// FailureOnExternalErrorPolicy treats every error as a failure except for rejected execution errors.
var FailureOnExternalErrorPolicy = func(_ context.Context, err error) algorithm.Result {
	if err == nil {
		return algorithm.ResultSuccess
	}

	// Rejected execution errors should be ignored by the algorithm
	if err != nil && !goErrors.Is(err, executor.ErrRejectedExecution) {
		return algorithm.ResultFailure
	}

	return algorithm.ResultIgnore
}

// NoFailurePolicy never returns a failure, only success or ignore.
// This can be used to adapt only on RTT/latency without considering errors.
var NoFailurePolicy = func(_ context.Context, err error) algorithm.Result {
	if err == nil {
		return algorithm.ResultSuccess
	}

	return algorithm.ResultIgnore
}

// FailureOnRejectedPolicy treats execution rejection as a failure.
var FailureOnRejectedPolicy = func(_ context.Context, err error) algorithm.Result {
	if err == nil {
		return algorithm.ResultSuccess
	}

	if err != nil && goErrors.Is(err, executor.ErrRejectedExecution) {
		return algorithm.ResultFailure
	}

	return algorithm.ResultIgnore
}
