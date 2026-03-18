# Go Adaptive Limiter

[![Go Reference](https://pkg.go.dev/badge/github.com/cansozeri/go-adaptive-limiter.svg)](https://pkg.go.dev/github.com/cansozeri/go-adaptive-limiter)
[![Go Report Card](https://goreportcard.com/badge/github.com/cansozeri/go-adaptive-limiter)](https://goreportcard.com/report/github.com/cansozeri/go-adaptive-limiter)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Adaptive concurrency limiter for Go based on TCP congestion control algorithms.

## Background

This library implements adaptive concurrency limiting inspired by [Netflix's concurrency-limits](https://github.com/Netflix/concurrency-limits) library for Java. Traditional rate limiting uses fixed RPS thresholds, which don't adapt to changing system conditions. This library uses Little's Law to dynamically adjust concurrency limits based on observed latency:

```
Concurrency Limit = Average RPS × Average Latency
```

The algorithms are based on proven TCP congestion control mechanisms (AIMD, Vegas) adapted for service-level concurrency management.

## Installation

```bash
go get github.com/cansozeri/go-adaptive-limiter
```

## Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/cansozeri/go-adaptive-limiter"
    "github.com/cansozeri/go-adaptive-limiter/algorithm"
    "github.com/cansozeri/go-adaptive-limiter/executor"
)

func main() {
    lim := limiter.New(
        limiter.WithAlgorithm(algorithm.NewAIMD(algorithm.AIMDConfig{
            MinimumLimit: 10,
            RTTTimeout: 100 * time.Millisecond,
        })),
    )
    
    err := lim.Execute(context.Background(), func() error {
        // Your business logic
        return doWork()
    })
}
```

## Algorithms

### AIMD (Additive Increase, Multiplicative Decrease)

Based on the TCP congestion control algorithm. Increases limit linearly, decreases multiplicatively on congestion.

```go
algorithm.NewAIMD(algorithm.AIMDConfig{
    MinimumLimit:       10,
    SlowStartThreshold: 50,
    RTTTimeout:         100 * time.Millisecond,
    BackoffRatio:       0.9,
})
```

Best for general-purpose adaptive limiting.

### Vegas

Based on TCP Vegas. Uses queue delay to detect congestion proactively.

```go
algorithm.NewVegas(algorithm.VegasConfig{
    MinimumLimit: 10,
    MaxLimit:     100,
    RttNoLoad:    50 * time.Millisecond,
})
```

Best for latency-sensitive services.

### Static

Fixed concurrency limit.

```go
algorithm.NewStatic(20)
```

Best for known capacity or testing.

## Executors

### FIFO

First-in-first-out queue with timeout.

```go
executor.NewFIFO(executor.FIFOConfig{
    MaxWaitTime: 1 * time.Second,
})
```

### LIFO

Last-in-first-out queue. Useful for prioritizing recent requests.

```go
executor.NewLIFO(executor.LIFOConfig{
    MaxWaitTime: 1 * time.Second,
    StopChannel: make(chan struct{}),
})
```

### Adaptive LIFO + CoDel

Implements [CoDel (Controlled Delay)](https://queue.acm.org/detail.cfm?id=2209336) with adaptive LIFO queue, based on [Facebook's implementation](https://queue.acm.org/detail.cfm?id=2839461).

```go
executor.NewAdaptiveLIFOCodel(executor.AdaptiveLIFOCodelConfig{
    CodelTargetDelay: 5 * time.Millisecond,
    CodelInterval:    100 * time.Millisecond,
    StopChannel:      make(chan struct{}),
})
```

Best for bufferbloat prevention.

## Usage Examples

### HTTP Server

```go
lim := limiter.New(
    limiter.WithAlgorithm(algorithm.NewVegas(algorithm.VegasConfig{
        MinimumLimit: 10,
        MaxLimit:     100,
    })),
)

http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
    err := lim.Execute(r.Context(), func() error {
        return handleRequest(r, w)
    })
    
    if err != nil {
        http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
    }
})
```

### With Return Value

```go
result, err := limiter.ExecuteWithResult(lim, ctx, func() (string, error) {
    data, err := fetchData()
    return data, err
})
```

### Monitoring

```go
stats := lim.Stats()
// stats.CurrentLimit - current concurrency limit
// stats.InFlight - admitted requests (queued + executing)
// stats.Executing - requests currently executing
```

## Result Policies

Control how execution results affect the limiter:

```go
// Count external errors as failures
limiter.WithResultPolicy(limiter.FailureOnExternalErrorPolicy)

// Adapt only on latency, ignore errors
limiter.WithResultPolicy(limiter.NoFailurePolicy)

// Count only rejections as failures
limiter.WithResultPolicy(limiter.FailureOnRejectedPolicy)
```

## Performance

Benchmarks on Apple M-series processor:

```
BenchmarkLimiter-8               828 ns/op    472 B/op     8 allocs/op
BenchmarkLimiterWithResult-8     967 ns/op    544 B/op    11 allocs/op
BenchmarkAIMDAlgorithm-8         958 ns/op    472 B/op     8 allocs/op
BenchmarkVegasAlgorithm-8       2364 ns/op    704 B/op    11 allocs/op
```

The limiter adds sub-microsecond overhead for AIMD and FIFO configurations. Recent optimizations improved Vegas algorithm performance by ~48%.

## Thread Safety

All operations are thread-safe:
- Lock-free atomic counters for in-flight tracking
- Mutex-protected algorithm state updates
- Safe for concurrent use from multiple goroutines

## Graceful Shutdown

```go
lim := limiter.New()
defer lim.Shutdown()

// Stops accepting new work immediately and waits for admitted work to finish.
// New Execute calls return limiter.ErrRejectedExecution after shutdown begins.
```

## Attribution

This library is inspired by:

- [Netflix's concurrency-limits library](https://github.com/Netflix/concurrency-limits) for Java
- TCP congestion control algorithms (AIMD, Vegas)
- [CoDel algorithm](https://queue.acm.org/detail.cfm?id=2209336) and [Facebook's adaptive LIFO implementation](https://queue.acm.org/detail.cfm?id=2839461)
- [Little's Law](https://en.wikipedia.org/wiki/Little%27s_law) for queueing theory

## Requirements

- Go 1.23 or later

## License

MIT - see [LICENSE](LICENSE) file.

## Contributing

Contributions are welcome. Please open an issue for discussion before submitting significant changes.

## Author

Can Sözeri - [GitHub](https://github.com/cansozeri)
