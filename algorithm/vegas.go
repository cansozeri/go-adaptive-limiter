package algorithm

import (
	stdlib "math"
	"math/rand"
	"sync"
	"time"

	mathfn "github.com/cansozeri/go-adaptive-limiter/internal/mathfn"
)

// Note: math/rand top-level functions are automatically thread-safe since Go 1.20.
// No additional synchronization needed for rand.Float64() calls.

// VegasConfig configures a limiter based on TCP Vegas where the limit increases by alpha if the queue_use is
// small (< alpha) and decreases by alpha if the queue_use is large (> beta).
//
// Queue size is calculated using the formula,
//
//	queue_use = limit - BWE×RttNoLoad = limit × (1 - RttNoLoad/RTTactual)
//
// For traditional TCP Vegas alpha is typically 2-3 and beta is typically 4-6.  To allow for better growth and stability
// at higher limits we set alpha=Max(3, 10% of the current limit) and beta=Max(6, 20% of the current limit).
type VegasConfig struct {
	MinimumLimit    int
	RttNoLoad       time.Duration
	MaxLimit        int
	Smoothing       float64
	AlphaFunc       func(int) int
	BetaFunc        func(int) int
	ThresholdFunc   func(int) int
	IncreaseFunc    func(float64) float64
	DecreaseFunc    func(float64) float64
	ProbeMultiplier int
	probeCount      int64
	probeJitter     float64
}

func (c *VegasConfig) defaults() {
	if c.MinimumLimit < 1 {
		c.MinimumLimit = 50
	}

	if c.MaxLimit < 1 {
		c.MaxLimit = 1000
	}

	if c.RttNoLoad == 0 {
		c.RttNoLoad = 2 * time.Second
	}

	if c.Smoothing < 1 || c.Smoothing > 1.0 {
		c.Smoothing = 1.0
	}

	if c.ProbeMultiplier <= 0 {
		c.ProbeMultiplier = 30
	}
	defaultLogFunc := mathfn.Log10RootFunction(c.MinimumLimit)
	if c.AlphaFunc == nil {
		c.AlphaFunc = func(limit int) int { return 3 * defaultLogFunc(limit) }
	}
	if c.BetaFunc == nil {
		c.BetaFunc = func(limit int) int { return 6 * defaultLogFunc(limit) }
	}
	if c.ThresholdFunc == nil {
		c.ThresholdFunc = func(limit int) int { return defaultLogFunc(limit) }
	}

	defaultLogFloatFunc := mathfn.Log10RootFloatFunction(float64(c.MinimumLimit))
	if c.IncreaseFunc == nil {
		c.IncreaseFunc = func(limit float64) float64 { return limit + defaultLogFloatFunc(limit) }
	}

	if c.DecreaseFunc == nil {
		c.DecreaseFunc = func(limit float64) float64 { return limit - defaultLogFloatFunc(limit) }
	}

	if c.ProbeMultiplier < 1 {
		c.ProbeMultiplier = 30
	}

	c.resetProbeJitter()
}

func NewVegas(cfg VegasConfig) Limiter {
	cfg.defaults()

	return &vegas{
		limit: float64(cfg.MinimumLimit),
		cfg:   cfg,
	}
}

type vegas struct {
	cfg   VegasConfig
	limit float64
	mu    sync.Mutex
}

func (c *VegasConfig) resetProbeJitter() {
	c.probeJitter = (Float64() / 2.0) + 0.5
}

func (v *vegas) MeasureSample(startTime time.Time, _ time.Duration, inflight int, result Result) int {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cfg.probeCount++
	rtt := time.Since(startTime)

	if v.shouldProbe() {
		v.cfg.resetProbeJitter()
		v.cfg.probeCount = 0
		v.cfg.RttNoLoad = rtt
		return int(v.limit)
	}

	if v.cfg.RttNoLoad == 0 || rtt < v.cfg.RttNoLoad {
		v.cfg.RttNoLoad = rtt
		return int(v.limit)
	}

	return v.updateEstimatedLimit(rtt, inflight, result)
}

func (v *vegas) GetLimit() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return int(v.limit)
}

func (v *vegas) shouldProbe() bool {
	return int64(v.cfg.probeJitter*float64(v.cfg.ProbeMultiplier)*v.limit) <= v.cfg.probeCount
}

func (v *vegas) updateEstimatedLimit(rtt time.Duration, inflight int, result Result) int {
	queueSize := int(stdlib.Ceil(v.limit * (1 - float64(v.cfg.RttNoLoad)/float64(rtt))))

	var newLimit float64
	currentLimit := int(v.limit)

	switch result {
	case ResultSuccess:
		alpha := v.cfg.AlphaFunc(currentLimit)
		beta := v.cfg.BetaFunc(currentLimit)
		threshold := v.cfg.ThresholdFunc(currentLimit)

		if queueSize < threshold {
			// Aggressive increase when no queuing
			newLimit = float64(currentLimit + beta)
		} else if queueSize < alpha {
			// Increase the limit if queue is still manageable
			newLimit = v.cfg.IncreaseFunc(v.limit)
		} else if queueSize > beta {
			// Detecting latency so decrease
			newLimit = v.cfg.DecreaseFunc(v.limit)
		} else {
			// otherwise we're within the sweet spot so nothing to do
			return currentLimit
		}

	case ResultFailure:
		if inflight*2 < currentLimit {
			return currentLimit
		}

		newLimit = v.cfg.DecreaseFunc(v.limit)
	}

	newLimit = stdlib.Max(1, stdlib.Min(float64(v.cfg.MaxLimit), newLimit))
	newLimit = (1-v.cfg.Smoothing)*v.limit + v.cfg.Smoothing*newLimit
	v.limit = newLimit
	return int(v.limit)
}

// Float64 generates a random float between 0 and 1 for jitter calculation.
// Uses math/rand top-level function which is automatically thread-safe in Go 1.20+.
// This is safe because jitter doesn't need cryptographic randomness.
func Float64() float64 {
	return rand.Float64()
}
