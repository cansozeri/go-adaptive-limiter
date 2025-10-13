package algorithm

import (
	"testing"
	"time"
)

func TestStatic_GetLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
	}{
		{"limit 10", 10},
		{"limit 50", 50},
		{"limit 100", 100},
		{"limit 1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg := NewStatic(tt.limit)
			if got := alg.GetLimit(); got != tt.limit {
				t.Errorf("GetLimit() = %d, want %d", got, tt.limit)
			}
		})
	}
}

func TestStatic_MeasureSample(t *testing.T) {
	limit := 42
	alg := NewStatic(limit)

	tests := []struct {
		name     string
		result   Result
		inflight int
	}{
		{"success", ResultSuccess, 10},
		{"failure", ResultFailure, 5},
		{"ignore", ResultIgnore, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLimit := alg.MeasureSample(time.Now(), 0, tt.inflight, tt.result)
			if newLimit != limit {
				t.Errorf("MeasureSample() = %d, want %d (limit should never change)", newLimit, limit)
			}
		})
	}
}

func TestStatic_AlwaysReturnsStaticLimit(t *testing.T) {
	limit := 25
	alg := NewStatic(limit)

	// Call MeasureSample many times with different results
	for i := 0; i < 100; i++ {
		result := ResultSuccess
		if i%2 == 0 {
			result = ResultFailure
		}
		newLimit := alg.MeasureSample(time.Now(), 0, i, result)
		if newLimit != limit {
			t.Errorf("iteration %d: limit changed to %d, expected %d", i, newLimit, limit)
		}
	}

	if alg.GetLimit() != limit {
		t.Errorf("GetLimit() = %d after multiple samples, want %d", alg.GetLimit(), limit)
	}
}
