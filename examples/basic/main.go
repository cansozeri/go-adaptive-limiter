// Basic example demonstrating standalone limiter usage
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cansozeri/go-adaptive-limiter"
	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

func main() {
	// Create a limiter with AIMD algorithm and FIFO executor
	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewAIMD(algorithm.AIMDConfig{
			MinimumLimit: 5,
			RTTTimeout:   100 * time.Millisecond,
			BackoffRatio: 0.9,
		})),
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{
			MaxWaitTime: 1 * time.Second,
		})),
	)

	// Execute with concurrency limiting
	for i := 0; i < 20; i++ {
		id := i
		go func() {
			err := lim.Execute(context.Background(), func() error {
				fmt.Printf("Executing job %d\n", id)
				time.Sleep(50 * time.Millisecond)
				return nil
			})

			if err != nil {
				fmt.Printf("Job %d rejected: %v\n", id, err)
			}
		}()
	}

	// Check stats
	time.Sleep(200 * time.Millisecond)
	stats := lim.Stats()
	fmt.Printf("\nLimiter Stats:\n")
	fmt.Printf("  Current Limit: %d\n", stats.CurrentLimit)
	fmt.Printf("  In Flight: %d\n", stats.InFlight)
	fmt.Printf("  Executing: %d\n", stats.Executing)

	time.Sleep(1 * time.Second)
	lim.Shutdown()
}

