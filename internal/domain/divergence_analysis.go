package domain

import "fmt"

// repeatedViolationThreshold is the axis score above which a result counts as a violation.
const repeatedViolationThreshold = 50

// CollectRepeatedViolations analyzes a slice of recent check results and returns
// any integrity axis that exceeded the violation threshold in ALL of the provided results.
// Returns nil when results is empty or no axis qualifies.
func CollectRepeatedViolations(results []CheckResult) []RepeatedViolation {
	if len(results) == 0 {
		return nil
	}

	// Count how many results have each axis above threshold
	countAbove := make(map[Axis]int)
	latestDetails := make(map[Axis]string)
	for _, r := range results {
		for axis, score := range r.Axes {
			if score.Score > repeatedViolationThreshold {
				countAbove[axis]++
				latestDetails[axis] = score.Details
			}
		}
	}

	var violations []RepeatedViolation
	for axis, count := range countAbove {
		if count == len(results) {
			violations = append(violations, RepeatedViolation{
				Axis:        string(axis),
				Description: latestDetails[axis],
				Count:       count,
			})
		}
	}
	return violations
}

// divergenceTrendStableThreshold is the maximum delta considered "stable".
const divergenceTrendStableThreshold = 5.0

// AnalyzeDivergenceTrend computes the trend direction of divergence scores across
// recent check results. Returns nil when fewer than 2 results are provided.
func AnalyzeDivergenceTrend(results []CheckResult) *DivergenceTrend {
	if len(results) < 2 {
		return nil
	}

	first := results[0].Divergence
	last := results[len(results)-1].Divergence
	delta := last - first

	var class DivergenceTrendClass
	var msg string
	switch {
	case delta > divergenceTrendStableThreshold:
		class = DivergenceTrendWorsening
		msg = fmt.Sprintf("Divergence increased by %.1f over %d checks (%.1f -> %.1f)", delta, len(results), first, last)
	case delta < -divergenceTrendStableThreshold:
		class = DivergenceTrendImproving
		msg = fmt.Sprintf("Divergence decreased by %.1f over %d checks (%.1f -> %.1f)", -delta, len(results), first, last)
	default:
		class = DivergenceTrendStable
		msg = fmt.Sprintf("Divergence stable (delta %.1f) over %d checks", delta, len(results))
	}

	return &DivergenceTrend{
		Class:   class,
		Delta:   delta,
		Message: msg,
	}
}
