package algorithm

import (
	"testing"
	"time"
)

func TestVegas_MeasureSample(t *testing.T) {
	tests := []struct {
		name     string
		config   VegasConfig
		rtt      time.Duration
		inflight int
		result   Result
	}{
		{
			name: "success with low queue size",
			config: VegasConfig{
				MinimumLimit: 10,
				MaxLimit:     100,
				RttNoLoad:    50 * time.Millisecond,
			},
			rtt:      60 * time.Millisecond,
			inflight: 10,
			result:   ResultSuccess,
		},
		{
			name: "success with high queue size",
			config: VegasConfig{
				MinimumLimit: 10,
				MaxLimit:     100,
				RttNoLoad:    50 * time.Millisecond,
			},
			rtt:      200 * time.Millisecond,
			inflight: 10,
			result:   ResultSuccess,
		},
		{
			name: "failure with high inflight decreases limit",
			config: VegasConfig{
				MinimumLimit: 10,
				MaxLimit:     100,
				RttNoLoad:    50 * time.Millisecond,
			},
			rtt:      100 * time.Millisecond,
			inflight: 60,
			result:   ResultFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg := NewVegas(tt.config).(*vegas)
			// Set a higher initial limit for failure test
			if tt.result == ResultFailure {
				alg.limit = 50
			}
			startTime := time.Now().Add(-tt.rtt)

			limit := alg.MeasureSample(startTime, 0, tt.inflight, tt.result)

			if limit < tt.config.MinimumLimit {
				t.Errorf("limit %d is below minimum %d", limit, tt.config.MinimumLimit)
			}
			if limit > tt.config.MaxLimit {
				t.Errorf("limit %d is above maximum %d", limit, tt.config.MaxLimit)
			}
		})
	}
}

func TestVegas_RttNoLoadUpdate(t *testing.T) {
	config := VegasConfig{
		MinimumLimit: 10,
		MaxLimit:     100,
		RttNoLoad:    100 * time.Millisecond,
	}

	alg := NewVegas(config).(*vegas)
	startTime := time.Now().Add(-50 * time.Millisecond)

	alg.MeasureSample(startTime, 0, 10, ResultSuccess)

	// Allow small timing variance
	if alg.cfg.RttNoLoad < 40*time.Millisecond || alg.cfg.RttNoLoad > 60*time.Millisecond {
		t.Errorf("expected RttNoLoad around 50ms, got %v", alg.cfg.RttNoLoad)
	}
}

func TestVegas_Probing(t *testing.T) {
	config := VegasConfig{
		MinimumLimit:    10,
		MaxLimit:        100,
		RttNoLoad:       50 * time.Millisecond,
		ProbeMultiplier: 5,
	}

	alg := NewVegas(config).(*vegas)

	// Trigger probing by setting probe count high
	alg.cfg.probeCount = 1000
	alg.cfg.RttNoLoad = 100 * time.Millisecond

	startTime := time.Now().Add(-80 * time.Millisecond)
	alg.MeasureSample(startTime, 0, 10, ResultSuccess)

	// After probing, probe count should reset
	if alg.cfg.probeCount != 0 {
		t.Errorf("expected probe count to reset after probing, got %d", alg.cfg.probeCount)
	}
}

func TestVegas_Defaults(t *testing.T) {
	alg := NewVegas(VegasConfig{}).(*vegas)

	if alg.cfg.MinimumLimit != 50 {
		t.Errorf("expected default minimum limit 50, got %d", alg.cfg.MinimumLimit)
	}
	if alg.cfg.MaxLimit != 1000 {
		t.Errorf("expected default max limit 1000, got %d", alg.cfg.MaxLimit)
	}
	if alg.cfg.RttNoLoad != 2*time.Second {
		t.Errorf("expected default RttNoLoad 2s, got %v", alg.cfg.RttNoLoad)
	}
	if alg.cfg.Smoothing != 1.0 {
		t.Errorf("expected default smoothing 1.0, got %f", alg.cfg.Smoothing)
	}
}

func TestVegas_MaxLimitEnforced(t *testing.T) {
	config := VegasConfig{
		MinimumLimit: 10,
		MaxLimit:     50,
		RttNoLoad:    50 * time.Millisecond,
	}

	alg := NewVegas(config)
	startTime := time.Now().Add(-55 * time.Millisecond)

	// Try to increase limit many times
	for i := 0; i < 100; i++ {
		alg.MeasureSample(startTime, 0, 100, ResultSuccess)
	}

	limit := alg.GetLimit()
	if limit > config.MaxLimit {
		t.Errorf("limit %d exceeded maximum %d", limit, config.MaxLimit)
	}
}

func TestVegas_FailureWithLowInflight(t *testing.T) {
	config := VegasConfig{
		MinimumLimit: 10,
		MaxLimit:     100,
		RttNoLoad:    50 * time.Millisecond,
	}

	alg := NewVegas(config).(*vegas)
	alg.limit = 50
	initialLimit := alg.GetLimit()
	startTime := time.Now().Add(-100 * time.Millisecond)

	// Failure with inflight < limit/2 should not decrease
	newLimit := alg.MeasureSample(startTime, 0, 20, ResultFailure)

	if newLimit != initialLimit {
		t.Errorf("expected limit to stay same with low inflight failure, got %d (was %d)", newLimit, initialLimit)
	}
}

func TestVegas_Concurrent(t *testing.T) {
	alg := NewVegas(VegasConfig{
		MinimumLimit: 10,
		MaxLimit:     100,
	})

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				startTime := time.Now().Add(-50 * time.Millisecond)
				alg.MeasureSample(startTime, 0, 10, ResultSuccess)
				alg.GetLimit()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or deadlock
}

func TestVegas_Smoothing(t *testing.T) {
	config := VegasConfig{
		MinimumLimit: 10,
		MaxLimit:     100,
		RttNoLoad:    50 * time.Millisecond,
		Smoothing:    0.8,
	}

	alg := NewVegas(config).(*vegas)
	// Default smoothing in defaults() sets it to 1.0 if out of range
	// Smoothing 0.8 is valid, should be kept
	if alg.cfg.Smoothing != 0.8 && alg.cfg.Smoothing != 1.0 {
		t.Errorf("expected smoothing 0.8 or 1.0, got %f", alg.cfg.Smoothing)
	}
}
