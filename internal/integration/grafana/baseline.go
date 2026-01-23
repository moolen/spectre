package grafana

import (
	"fmt"
	"math"
	"sort"
	"time"

	"gonum.org/v1/gonum/stat"
)

// Baseline represents statistical baseline for a metric
type Baseline struct {
	MetricName  string
	Mean        float64
	StdDev      float64
	SampleCount int
	WindowHour  int
	DayType     string // "weekday" or "weekend"
}

// MetricAnomaly represents a detected anomaly in a metric
type MetricAnomaly struct {
	MetricName string
	Value      float64
	Baseline   float64
	ZScore     float64
	Severity   string // "info", "warning", "critical"
	Timestamp  time.Time
}

// StateDistribution represents the percentage of time spent in each alert state
type StateDistribution struct {
	PercentNormal  float64 // 0.0-1.0
	PercentPending float64 // 0.0-1.0
	PercentFiring  float64 // 0.0-1.0
}

// InsufficientDataError indicates that there is not enough historical data
// to compute a reliable baseline
type InsufficientDataError struct {
	Available time.Duration
	Required  time.Duration
}

func (e *InsufficientDataError) Error() string {
	return fmt.Sprintf("insufficient data for baseline: available %v, required %v",
		e.Available, e.Required)
}

// ComputeRollingBaseline calculates the baseline state distribution and standard deviation
// from historical state transitions over a lookback period.
//
// Uses Last Observation Carried Forward (LOCF) interpolation to fill gaps in data.
// Requires at least 24 hours of history; returns error if insufficient.
//
// Parameters:
//   - transitions: historical state transitions (should span lookbackDays)
//   - lookbackDays: number of days to analyze (typically 7)
//   - currentTime: end of analysis window
//
// Returns:
//   - baseline: average state distribution across available days
//   - stdDev: sample standard deviation of firing percentage across days
//   - error: InsufficientDataError if < 24h history available
func ComputeRollingBaseline(transitions []StateTransition, lookbackDays int, currentTime time.Time) (StateDistribution, float64, error) {
	lookbackDuration := time.Duration(lookbackDays) * 24 * time.Hour
	windowStart := currentTime.Add(-lookbackDuration)

	// Sort transitions chronologically
	sortedTransitions := make([]StateTransition, len(transitions))
	copy(sortedTransitions, transitions)
	sort.Slice(sortedTransitions, func(i, j int) bool {
		return sortedTransitions[i].Timestamp.Before(sortedTransitions[j].Timestamp)
	})

	// Find first transition in or before window
	var relevantTransitions []StateTransition
	var initialState string = "normal" // Assume normal if no prior history
	for i, t := range sortedTransitions {
		if !t.Timestamp.Before(windowStart) {
			// This transition is at or after window start (in window)
			if i > 0 {
				// Use the ToState from previous transition as initial state
				initialState = sortedTransitions[i-1].ToState
			}
			relevantTransitions = append(relevantTransitions, t)
		} else if i == len(sortedTransitions)-1 || !sortedTransitions[i+1].Timestamp.Before(windowStart) {
			// This is the last transition before window - use its ToState
			initialState = t.ToState
		}
	}

	// Check if we have enough data
	// If we have transitions spanning at least 24 hours, or we know the initial state
	// from before the window, we can compute a baseline using LOCF
	var dataStart time.Time
	if len(sortedTransitions) > 0 && sortedTransitions[0].Timestamp.Before(windowStart) {
		// We have data from before the window, so we know the initial state for full window
		dataStart = windowStart
	} else if len(relevantTransitions) > 0 {
		// Use the first transition in window as data start
		dataStart = relevantTransitions[0].Timestamp
	} else {
		// No transitions at all - assume we have the full window of stable state
		dataStart = windowStart
	}

	// Check if we have at least 24 hours of data coverage
	// The data span is from the earliest known state to current time
	availableDuration := currentTime.Sub(dataStart)
	if availableDuration < 24*time.Hour {
		return StateDistribution{}, 0.0, &InsufficientDataError{
			Available: availableDuration,
			Required:  24 * time.Hour,
		}
	}

	// Compute daily distributions using LOCF
	dailyDistributions := computeDailyDistributions(initialState, relevantTransitions, windowStart, currentTime, lookbackDays)

	// Calculate average distribution
	var totalNormal, totalPending, totalFiring float64
	firingPercentages := make([]float64, 0, len(dailyDistributions))

	for _, dist := range dailyDistributions {
		totalNormal += dist.PercentNormal
		totalPending += dist.PercentPending
		totalFiring += dist.PercentFiring
		firingPercentages = append(firingPercentages, dist.PercentFiring)
	}

	numDays := float64(len(dailyDistributions))
	baseline := StateDistribution{
		PercentNormal:  totalNormal / numDays,
		PercentPending: totalPending / numDays,
		PercentFiring:  totalFiring / numDays,
	}

	// Calculate sample standard deviation of firing percentage
	var stdDev float64
	if len(firingPercentages) >= 2 {
		stdDev = stat.StdDev(firingPercentages, nil)
	}

	return baseline, stdDev, nil
}

// computeDailyDistributions splits the time window into daily buckets and computes
// state distribution for each day using LOCF interpolation
func computeDailyDistributions(initialState string, transitions []StateTransition, windowStart, windowEnd time.Time, lookbackDays int) []StateDistribution {
	var distributions []StateDistribution
	currentState := initialState

	for day := 0; day < lookbackDays; day++ {
		dayStart := windowStart.Add(time.Duration(day) * 24 * time.Hour)
		dayEnd := dayStart.Add(24 * time.Hour)

		// Don't go past the window end
		if dayStart.After(windowEnd) {
			break
		}
		if dayEnd.After(windowEnd) {
			dayEnd = windowEnd
		}

		dist, endState := computeStateDistributionForPeriod(currentState, transitions, dayStart, dayEnd)
		distributions = append(distributions, dist)

		// Update state for next day
		currentState = endState
	}

	return distributions
}

// computeStateDistributionForPeriod calculates the percentage of time spent in each state
// during a specific time period using LOCF interpolation.
// Returns the distribution and the ending state for LOCF continuation.
func computeStateDistributionForPeriod(initialState string, transitions []StateTransition, periodStart, periodEnd time.Time) (StateDistribution, string) {
	var normalDuration, pendingDuration, firingDuration time.Duration

	currentState := initialState
	currentTime := periodStart

	// Process each transition in the period
	for _, t := range transitions {
		if t.Timestamp.After(periodEnd) {
			break
		}

		if !t.Timestamp.Before(periodStart) && !t.Timestamp.After(periodEnd) {
			// Transition is within period (inclusive of periodStart, exclusive of periodEnd)
			// Add duration in current state until this transition
			if t.Timestamp.After(currentTime) {
				duration := t.Timestamp.Sub(currentTime)
				addDurationToState(&normalDuration, &pendingDuration, &firingDuration, currentState, duration)
				currentTime = t.Timestamp
			}

			// Update state
			currentState = t.ToState
		}
	}

	// Add remaining time in final state until period end
	if currentTime.Before(periodEnd) {
		duration := periodEnd.Sub(currentTime)
		addDurationToState(&normalDuration, &pendingDuration, &firingDuration, currentState, duration)
	}

	// Convert to percentages
	totalDuration := periodEnd.Sub(periodStart)
	if totalDuration == 0 {
		return StateDistribution{PercentNormal: 1.0}, currentState
	}

	dist := StateDistribution{
		PercentNormal:  float64(normalDuration) / float64(totalDuration),
		PercentPending: float64(pendingDuration) / float64(totalDuration),
		PercentFiring:  float64(firingDuration) / float64(totalDuration),
	}

	return dist, currentState
}

// addDurationToState adds duration to the appropriate state counter
func addDurationToState(normalDuration, pendingDuration, firingDuration *time.Duration, state string, duration time.Duration) {
	switch state {
	case "normal":
		*normalDuration += duration
	case "pending":
		*pendingDuration += duration
	case "firing":
		*firingDuration += duration
	}
}

// CompareToBaseline computes how many standard deviations the current state distribution
// is from the baseline, focusing on the firing percentage.
//
// Parameters:
//   - current: current state distribution
//   - baseline: historical baseline state distribution
//   - stdDev: standard deviation of firing percentage from baseline computation
//
// Returns:
//   - deviationScore: number of standard deviations from baseline (absolute value)
//     A score of 2.0 indicates the current firing percentage is 2σ from baseline
func CompareToBaseline(current, baseline StateDistribution, stdDev float64) float64 {
	// Avoid division by zero
	if stdDev == 0.0 {
		return 0.0
	}

	// Calculate absolute deviation in firing percentage
	deviation := math.Abs(current.PercentFiring - baseline.PercentFiring)

	// Convert to number of standard deviations
	return deviation / stdDev
}
