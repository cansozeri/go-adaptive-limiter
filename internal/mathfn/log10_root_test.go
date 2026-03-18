package math

import "testing"

func TestLog10RootFunction_ReferenceValues(t *testing.T) {
	fn := Log10RootFunction(0)

	tests := []struct {
		input int
		want  int
	}{
		{1, 1},
		{9, 1},
		{10, 1},
		{50, 1},
		{99, 1},
		{100, 2},
		{500, 2},
		{999, 2},
		{1000, 3},
		{5000, 3},
	}

	for _, tt := range tests {
		if got := fn(tt.input); got != tt.want {
			t.Errorf("Log10RootFunction(0)(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLog10RootFunction_WithBaseline(t *testing.T) {
	fn := Log10RootFunction(10)

	tests := []struct {
		input int
		want  int
	}{
		{10, 11},
		{100, 12},
		{1000, 13},
	}

	for _, tt := range tests {
		if got := fn(tt.input); got != tt.want {
			t.Errorf("Log10RootFunction(10)(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLog10RootFunction_ZeroBaseline_VegasThresholds(t *testing.T) {
	fn := Log10RootFunction(0)

	// With baseline=0, the function produces the values used by Vegas:
	// alpha(limit) = 3 * fn(limit), beta(limit) = 6 * fn(limit)
	tests := []struct {
		limit     int
		wantAlpha int
		wantBeta  int
	}{
		{10, 3, 6},
		{50, 3, 6},
		{100, 6, 12},
		{500, 6, 12},
		{1000, 9, 18},
	}

	for _, tt := range tests {
		base := fn(tt.limit)
		alpha := 3 * base
		beta := 6 * base
		if alpha != tt.wantAlpha {
			t.Errorf("limit=%d: alpha = 3*fn(%d) = %d, want %d", tt.limit, tt.limit, alpha, tt.wantAlpha)
		}
		if beta != tt.wantBeta {
			t.Errorf("limit=%d: beta = 6*fn(%d) = %d, want %d", tt.limit, tt.limit, beta, tt.wantBeta)
		}
	}
}

func TestLog10RootFloatFunction_Values(t *testing.T) {
	fn := Log10RootFloatFunction(0)

	tests := []struct {
		input float64
		want  float64
	}{
		{10, 1},
		{100, 2},
		{1000, 3},
	}

	for _, tt := range tests {
		if got := fn(tt.input); got != tt.want {
			t.Errorf("Log10RootFloatFunction(0)(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLog10RootFunction_LargeValues(t *testing.T) {
	fn := Log10RootFunction(0)

	// Values >= 1000 use direct math.Log10 instead of lookup table
	tests := []struct {
		input int
		want  int
	}{
		{1000, 3},
		{10000, 4},
		{100000, 5},
	}

	for _, tt := range tests {
		if got := fn(tt.input); got != tt.want {
			t.Errorf("Log10RootFunction(0)(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
