package limiter_test

import (
	"context"
	"testing"

	"github.com/cansozeri/go-adaptive-limiter"
	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

func BenchmarkLimiter(b *testing.B) {
	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(100)),
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{})),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = lim.Execute(context.Background(), func() error {
				return nil
			})
		}
	})
}

func BenchmarkLimiterWithResult(b *testing.B) {
	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewStatic(100)),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = limiter.ExecuteWithResult(lim, context.Background(), func() (int, error) {
				return 42, nil
			})
		}
	})
}

func BenchmarkAIMDAlgorithm(b *testing.B) {
	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewAIMD(algorithm.AIMDConfig{
			MinimumLimit: 10,
		})),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lim.Execute(context.Background(), func() error {
			return nil
		})
	}
}

func BenchmarkVegasAlgorithm(b *testing.B) {
	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewVegas(algorithm.VegasConfig{
			MinimumLimit: 10,
			MaxLimit:     100,
		})),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lim.Execute(context.Background(), func() error {
			return nil
		})
	}
}

