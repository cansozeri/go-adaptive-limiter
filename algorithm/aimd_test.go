package algorithm

import (
	"testing"
	"time"
)

func TestAIMD_MeasureSample_Success(t *testing.T) {
	tests := []struct {
		name           string
		config         AIMDConfig
		initialLimit   int
		rtt            time.Duration
		inflight       int
		result         Result
		expectIncrease bool
		expectDecrease bool
	}{
		{
			name: "success below timeout increases limit",
			config: AIMDConfig{
				MinimumLimit: 10,
				RTTTimeout:   100 * time.Millisecond,
				BackoffRatio: 0.9,
			},
			rtt:            50 * time.Millisecond,
			inflight:       20,
			result:         ResultSuccess,
			expectIncrease: true,
		},
		{
			name: "success above timeout decreases limit",
			config: AIMDConfig{
				MinimumLimit: 10,
				RTTTimeout:   100 * time.Millisecond,
				BackoffRatio: 0.9,
			},
			initialLimit:   50,
			rtt:            150 * time.Millisecond,
			inflight:       10,
			result:         ResultSuccess,
			expectDecrease: true,
		},
		{
			name: "failure always decreases",
			config: AIMDConfig{
				MinimumLimit: 10,
				RTTTimeout:   100 * time.Millisecond,
				BackoffRatio: 0.9,
			},
			initialLimit:   50,
			rtt:            50 * time.Millisecond,
			inflight:       10,
			result:         ResultFailure,
			expectDecrease: true,
		},
		{
			name: "ignore returns current limit",
			config: AIMDConfig{
				MinimumLimit: 10,
			},
			rtt:      50 * time.Millisecond,
			inflight: 10,
			result:   ResultIgnore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg := NewAIMD(tt.config).(*aimd)
			if tt.initialLimit > 0 {
				alg.limit = float64(tt.initialLimit)
			}
			initialLimit := alg.GetLimit()
			startTime := time.Now().Add(-tt.rtt)

			newLimit := alg.MeasureSample(startTime, 0, tt.inflight, tt.result)

			if tt.expectIncrease && newLimit <= initialLimit {
				t.Errorf("expected limit to increase, got %d (was %d)", newLimit, initialLimit)
			}
			if tt.expectDecrease && newLimit >= initialLimit {
				t.Errorf("expected limit to decrease, got %d (was %d)", newLimit, initialLimit)
			}
			if !tt.expectIncrease && !tt.expectDecrease && newLimit != initialLimit {
				t.Errorf("expected limit to stay same, got %d (was %d)", newLimit, initialLimit)
			}
		})
	}
}

func TestAIMD_MinimumLimit(t *testing.T) {
	config := AIMDConfig{
		MinimumLimit: 10,
		BackoffRatio: 0.5,
		RTTTimeout:   100 * time.Millisecond,
	}

	alg := NewAIMD(config)
	startTime := time.Now().Add(-200 * time.Millisecond)

	// Decrease limit multiple times
	for i := 0; i < 20; i++ {
		alg.MeasureSample(startTime, 0, 5, ResultFailure)
	}

	limit := alg.GetLimit()
	if limit < config.MinimumLimit {
		t.Errorf("limit went below minimum: got %d, minimum %d", limit, config.MinimumLimit)
	}
}

func TestAIMD_SlowStart(t *testing.T) {
	config := AIMDConfig{
		MinimumLimit:       10,
		SlowStartThreshold: 20,
		RTTTimeout:         100 * time.Millisecond,
		BackoffRatio:       0.9,
	}

	alg := NewAIMD(config)
	startTime := time.Now().Add(-50 * time.Millisecond)

	// Should increase by 1 during slow start
	initialLimit := alg.GetLimit()
	newLimit := alg.MeasureSample(startTime, 0, 50, ResultSuccess)

	if newLimit != initialLimit+1 {
		t.Errorf("expected slow start increment of 1, got difference of %d", newLimit-initialLimit)
	}

	// Increase beyond slow start threshold
	for i := 0; i < 15; i++ {
		alg.MeasureSample(startTime, 0, 50, ResultSuccess)
	}

	// Now should increase slower
	beforeLimit := alg.GetLimit()
	afterLimit := alg.MeasureSample(startTime, 0, 50, ResultSuccess)

	if afterLimit-beforeLimit >= 1 {
		t.Errorf("expected fractional increase after slow start, got %d", afterLimit-beforeLimit)
	}
}

func TestAIMD_Defaults(t *testing.T) {
	alg := NewAIMD(AIMDConfig{})
	limit := alg.GetLimit()

	if limit != 10 {
		t.Errorf("expected default minimum limit 10, got %d", limit)
	}
}

func TestAIMD_Concurrent(t *testing.T) {
	alg := NewAIMD(AIMDConfig{
		MinimumLimit: 10,
		RTTTimeout:   100 * time.Millisecond,
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

func TestAIMD_BackoffRatio(t *testing.T) {
	tests := []struct {
		name         string
		backoffRatio float64
		expected     float64
	}{
		{"valid ratio", 0.9, 0.9},
		{"lower bound", 0.5, 0.5},
		{"too low", 0.3, 0.9},
		{"too high", 1.5, 0.9},
		{"exactly one is useless", 1.0, 0.9},
		{"zero", 0, 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AIMDConfig{
				MinimumLimit: 100,
				BackoffRatio: tt.backoffRatio,
			}
			alg := NewAIMD(config).(*aimd)

			if alg.cfg.BackoffRatio != tt.expected {
				t.Errorf("expected backoff ratio %.1f, got %.1f", tt.expected, alg.cfg.BackoffRatio)
			}
		})
	}
}

func TestAIMD_AppLimitedNoIncrease(t *testing.T) {
	config := AIMDConfig{
		MinimumLimit: 10,
		RTTTimeout:   100 * time.Millisecond,
		BackoffRatio: 0.9,
	}

	alg := NewAIMD(config).(*aimd)
	alg.limit = 20
	initialLimit := alg.GetLimit()
	startTime := time.Now().Add(-50 * time.Millisecond)

	// inflight=3, well below 50% utilization.
	// Should not increase: the system is not under real load.
	newLimit := alg.MeasureSample(startTime, 0, 3, ResultSuccess)

	if newLimit != initialLimit {
		t.Errorf("app-limited success should not change limit: got %d, was %d", newLimit, initialLimit)
	}
}

func TestAIMD_IncreasesWhenUtilized(t *testing.T) {
	config := AIMDConfig{
		MinimumLimit: 10,
		RTTTimeout:   100 * time.Millisecond,
		BackoffRatio: 0.9,
	}

	alg := NewAIMD(config).(*aimd)
	alg.limit = 20
	initialLimit := alg.GetLimit()
	startTime := time.Now().Add(-50 * time.Millisecond)

	// inflight=15, utilization is 75%. System is sufficiently loaded
	// to trust the signal and allow growth.
	newLimit := alg.MeasureSample(startTime, 0, 15, ResultSuccess)

	if newLimit <= initialLimit {
		t.Errorf("success with adequate inflight should increase limit: got %d, was %d", newLimit, initialLimit)
	}
}

func TestAIMD_MaxLimitEnforced(t *testing.T) {
	config := AIMDConfig{
		MinimumLimit: 10,
		MaxLimit:     50,
		RTTTimeout:   1 * time.Second,
		BackoffRatio: 0.9,
	}

	alg := NewAIMD(config)
	startTime := time.Now().Add(-50 * time.Millisecond)

	for i := 0; i < 200; i++ {
		alg.MeasureSample(startTime, 0, 200, ResultSuccess)
	}

	limit := alg.GetLimit()
	if limit > config.MaxLimit {
		t.Errorf("limit %d exceeded maximum %d", limit, config.MaxLimit)
	}
}

func TestAIMD_MaxLimitDefault(t *testing.T) {
	alg := NewAIMD(AIMDConfig{}).(*aimd)

	if alg.cfg.MaxLimit != 200 {
		t.Errorf("expected default max limit 200, got %d", alg.cfg.MaxLimit)
	}
}
