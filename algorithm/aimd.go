package algorithm

import (
	"sync"
	"time"
)

// AIMDConfig is the configuration of the algorithm used for the AIMD adaptive algorithm.
type AIMDConfig struct {
	// MinimumLimit is the minimum limit the algorithm will decrease to. It also will be the starting limit.
	MinimumLimit int
	// This is like TCP algorithm's `ssthresh`. It will start increasing the limit by one
	// and when this threshold is reached it will change mode and increase slowly.
	// If set to 0 then slow start will be disabled.
	SlowStartThreshold int
	// RTTTimeout is the threshold above which RTT will be measured as a failure. This is an important setting
	// that depends on the application. Default is 2s but your app may need a greater or lesser timeout.
	RTTTimeout time.Duration
	// BackoffRatio is the ratio used to decrease the limit when a failure occurs.
	// The formula is: new limit = current limit * backoffRatio.
	BackoffRatio float64
	// LimitIncrementInflightFactor will increment the limit only if inflight * LimitIncrementInflightFactor > limit
	LimitIncrementInflightFactor int
}

func (c *AIMDConfig) defaults() {
	// Safety defaults.
	if c.BackoffRatio < 0.5 || c.BackoffRatio > 1 {
		c.BackoffRatio = 0.9
	}

	if c.RTTTimeout == 0 {
		c.RTTTimeout = 2 * time.Second
	}

	if c.MinimumLimit == 0 {
		c.MinimumLimit = 10
	}

	if c.LimitIncrementInflightFactor == 0 {
		c.LimitIncrementInflightFactor = 1
	}
}

// NewAIMD creates a new AIMD (Additive Increase/Multiplicative Decrease) adaptive limiter algorithm,
// based on the TCP congestion control algorithm. It increases the limit at a constant rate and
// decreases by a configured factor when congestion occurs.
// More info: https://en.wikipedia.org/wiki/Additive_increase/multiplicative_decrease
func NewAIMD(cfg AIMDConfig) Limiter {
	cfg.defaults()

	return &aimd{
		limit: float64(cfg.MinimumLimit),
		cfg:   cfg,
	}
}

type aimd struct {
	cfg   AIMDConfig
	limit float64
	mu    sync.Mutex
}

// MeasureSample satisfies Algorithm interface.
func (a *aimd) MeasureSample(startTime time.Time, _ time.Duration, inflight int, result Result) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	currentLimit := int(a.limit)
	switch result {
	case ResultSuccess:
		// Although we have a success maybe we are experiencing congestion.
		if time.Since(startTime) > a.cfg.RTTTimeout {
			return a.decreaseLimit()
		}

		// This is a real success.
		// Only increase if we need it. If not we would be increasing forever.
		// If we have double of inflight request waiting then increase.
		if inflight > currentLimit*a.cfg.LimitIncrementInflightFactor {
			return a.increaseLimit()
		}

	case ResultFailure:
		return a.decreaseLimit()

	}
	// Same as ignore.
	return currentLimit
}

// decreaseLimit will decrease the limit based on the backoff ratio.
func (a *aimd) decreaseLimit() int {
	a.limit = a.limit * a.cfg.BackoffRatio
	min := float64(a.cfg.MinimumLimit)
	if a.limit <= min {
		a.limit = min
	}
	return int(a.limit)
}

// increaseLimit will increase the limit being aware of slow start.
func (a *aimd) increaseLimit() int {
	// If slow start is disabled or our limit is less than the slow start threshold then
	// increment by one.
	if int(a.limit) < a.cfg.SlowStartThreshold || a.cfg.SlowStartThreshold == 0 {
		a.limit++
	} else {
		// Slow start threshold bypassed.
		a.limit = a.limit + (1 * (1 / a.limit))
	}

	return int(a.limit)
}

// GetLimit satisfies Algorithm interface.
func (a *aimd) GetLimit() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return int(a.limit)
}
