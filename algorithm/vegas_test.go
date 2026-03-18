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
			initialLimit := int(alg.limit)
			startTime := time.Now().Add(-tt.rtt)

			limit := alg.MeasureSample(startTime, 0, tt.inflight, tt.result)

			if limit < 1 {
				t.Errorf("limit %d is below hard minimum 1", limit)
			}
			if limit > tt.config.MaxLimit {
				t.Errorf("limit %d is above maximum %d", limit, tt.config.MaxLimit)
			}
			// High queue or failure should decrease the limit
			if tt.name == "success with high queue size" || tt.name == "failure with high inflight decreases limit" {
				if limit >= initialLimit {
					t.Errorf("expected limit to decrease from %d, got %d", initialLimit, limit)
				}
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

	// A drop is a drop: failure must always trigger decrease,
	// even when inflight is well below limit/2.
	newLimit := alg.MeasureSample(startTime, 0, 20, ResultFailure)

	if newLimit >= initialLimit {
		t.Errorf("failure should always decrease limit: got %d (was %d)", newLimit, initialLimit)
	}
}

func TestVegas_SuccessAppLimitedSkips(t *testing.T) {
	alg := NewVegas(VegasConfig{
		MinimumLimit: 100,
		MaxLimit:     1000,
		RttNoLoad:    50 * time.Millisecond,
	}).(*vegas)

	initialLimit := alg.GetLimit()
	startTime := time.Now().Add(-100 * time.Millisecond)

	// inflight*2 = 20 < limit 100: system is underutilized.
	// Adapting on sparse samples would distort the limit.
	newLimit := alg.MeasureSample(startTime, 0, 10, ResultSuccess)

	if newLimit != initialLimit {
		t.Errorf("app-limited success should not change limit: got %d (was %d)", newLimit, initialLimit)
	}
}

func TestVegas_SuccessiveFailuresDecrease(t *testing.T) {
	alg := NewVegas(VegasConfig{
		MinimumLimit: 10,
		MaxLimit:     1000,
		RttNoLoad:    50 * time.Millisecond,
	}).(*vegas)
	alg.limit = 100

	prevLimit := alg.GetLimit()
	for i := 0; i < 10; i++ {
		startTime := time.Now().Add(-100 * time.Millisecond)
		newLimit := alg.MeasureSample(startTime, 0, 200, ResultFailure)
		if newLimit > prevLimit {
			t.Errorf("iteration %d: limit increased from %d to %d during failure", i, prevLimit, newLimit)
		}
		prevLimit = newLimit
	}

	if prevLimit >= 100 {
		t.Errorf("expected limit to decrease after 10 failures, got %d", prevLimit)
	}
}

func TestVegas_LimitFloorAtOne(t *testing.T) {
	alg := NewVegas(VegasConfig{
		MinimumLimit: 1,
		MaxLimit:     100,
		RttNoLoad:    50 * time.Millisecond,
	}).(*vegas)
	alg.limit = 5

	for i := 0; i < 100; i++ {
		startTime := time.Now().Add(-100 * time.Millisecond)
		alg.MeasureSample(startTime, 0, 200, ResultFailure)
	}

	limit := alg.GetLimit()
	if limit < 1 {
		t.Errorf("limit must never go below 1, got %d", limit)
	}
}

func TestVegas_SmoothingEffect(t *testing.T) {
	tests := []struct {
		name      string
		smoothing float64
		wantLimit int
	}{
		{"full smoothing applies raw value", 1.0, 102},
		{"half smoothing blends old and new", 0.5, 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg := NewVegas(VegasConfig{
				MinimumLimit: 100,
				MaxLimit:     1000,
				RttNoLoad:    50 * time.Millisecond,
				Smoothing:    tt.smoothing,
			}).(*vegas)

			// Call updateEstimatedLimit directly to avoid timing variance.
			// limit=100, rtt=52ms: queueSize = ceil(100*(1-50/52)) = 4
			// alpha(100)=6, threshold(100)=2, queueSize(4) < alpha -> gradual increase
			// increaseFunc(100) = 102
			rtt := 52 * time.Millisecond
			limit := alg.updateEstimatedLimit(rtt, 60, ResultSuccess)

			if limit != tt.wantLimit {
				t.Errorf("smoothing=%.1f: got limit %d, want %d", tt.smoothing, limit, tt.wantLimit)
			}
		})
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
	if alg.cfg.Smoothing != 0.8 {
		t.Errorf("expected smoothing 0.8, got %f", alg.cfg.Smoothing)
	}
}

func TestVegas_DefaultAlphaBetaThreshold(t *testing.T) {
	// Default alpha/beta/threshold use log10-scaled values:
	//   alpha(limit)     = 3 * max(1, floor(log10(limit)))
	//   beta(limit)      = 6 * max(1, floor(log10(limit)))
	//   threshold(limit) = max(1, floor(log10(limit)))
	tests := []struct {
		limit     int
		wantAlpha int
		wantBeta  int
		wantThres int
	}{
		{limit: 10, wantAlpha: 3, wantBeta: 6, wantThres: 1},
		{limit: 100, wantAlpha: 6, wantBeta: 12, wantThres: 2},
		{limit: 500, wantAlpha: 6, wantBeta: 12, wantThres: 2},
		{limit: 1000, wantAlpha: 9, wantBeta: 18, wantThres: 3},
	}

	alg := NewVegas(VegasConfig{}).(*vegas)

	for _, tt := range tests {
		if got := alg.cfg.AlphaFunc(tt.limit); got != tt.wantAlpha {
			t.Errorf("AlphaFunc(%d) = %d, want %d", tt.limit, got, tt.wantAlpha)
		}
		if got := alg.cfg.BetaFunc(tt.limit); got != tt.wantBeta {
			t.Errorf("BetaFunc(%d) = %d, want %d", tt.limit, got, tt.wantBeta)
		}
		if got := alg.cfg.ThresholdFunc(tt.limit); got != tt.wantThres {
			t.Errorf("ThresholdFunc(%d) = %d, want %d", tt.limit, got, tt.wantThres)
		}
	}
}

func TestVegas_DefaultIncreaseDecrease(t *testing.T) {
	// Default increase/decrease should adjust by log10(limit):
	//   increase(limit) = limit + max(1, floor(log10(limit)))
	//   decrease(limit) = limit - max(1, floor(log10(limit)))
	tests := []struct {
		limit        float64
		wantIncrease float64
		wantDecrease float64
	}{
		{limit: 10, wantIncrease: 11, wantDecrease: 9},
		{limit: 100, wantIncrease: 102, wantDecrease: 98},
		{limit: 1000, wantIncrease: 1003, wantDecrease: 997},
	}

	alg := NewVegas(VegasConfig{}).(*vegas)

	for _, tt := range tests {
		if got := alg.cfg.IncreaseFunc(tt.limit); got != tt.wantIncrease {
			t.Errorf("IncreaseFunc(%v) = %v, want %v", tt.limit, got, tt.wantIncrease)
		}
		if got := alg.cfg.DecreaseFunc(tt.limit); got != tt.wantDecrease {
			t.Errorf("DecreaseFunc(%v) = %v, want %v", tt.limit, got, tt.wantDecrease)
		}
	}
}
