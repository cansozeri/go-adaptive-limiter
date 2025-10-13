package limiter

import (
	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

// Option configures a Limiter.
type Option func(*config)

type config struct {
	algorithm algorithm.Limiter
	executor  executor.Executor
	policy    ResultPolicy
}

func defaultConfig() *config {
	return &config{
		algorithm: algorithm.NewAIMD(algorithm.AIMDConfig{}),
		executor:  executor.NewFIFO(executor.FIFOConfig{}),
		policy:    FailureOnRejectedPolicy,
	}
}

// WithAlgorithm sets the concurrency limit algorithm.
// Available algorithms: AIMD, Vegas, Static.
func WithAlgorithm(alg algorithm.Limiter) Option {
	return func(c *config) {
		c.algorithm = alg
	}
}

// WithExecutor sets the executor for managing worker pools.
// Available executors: FIFO, LIFO, AdaptiveLIFOCodel.
func WithExecutor(exec executor.Executor) Option {
	return func(c *config) {
		c.executor = exec
	}
}

// WithResultPolicy sets the policy for categorizing execution results.
// Available policies: FailureOnExternalErrorPolicy, NoFailurePolicy, FailureOnRejectedPolicy.
func WithResultPolicy(policy ResultPolicy) Option {
	return func(c *config) {
		c.policy = policy
	}
}

