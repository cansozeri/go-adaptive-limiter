package math

import (
	"math"
)

var log10RootLookup []int

// Log10RootFunction is a specialized utility function used by limiters to calculate thresholds using log10 of the
// current algorithm.  Here we pre-compute the log10 root of numbers up to 1000 (or more) because the log10 root
// operation can be slow.
func Log10RootFunction(baseline int) func(minimumLimit int) int {
	return func(minimumLimit int) int {
		if minimumLimit < len(log10RootLookup) {
			return baseline + log10RootLookup[minimumLimit]
		}
		return baseline + int(math.Log10(float64(minimumLimit)))
	}
}

// Log10RootFloatFunction is a specialized utility function used by limiters to calculate thresholds using log10 of the
// current algorithm.  Here we pre-compute the log10 root of numbers up to 1000 (or more) because the log10 root
// operation can be slow.
func Log10RootFloatFunction(baseline float64) func(minimumLimit float64) float64 {
	return func(minimumLimit float64) float64 {
		if int(minimumLimit) < len(log10RootLookup) {
			return baseline + float64(log10RootLookup[int(minimumLimit)])
		}
		return baseline + math.Log10(minimumLimit)
	}
}
