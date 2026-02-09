package grafana

import (
	"time"
)

// FlappingDetector identifies alerts that are flapping and should be excluded
// from correlation analysis.
type FlappingDetector struct {
	maxTransitionsPerDay int
	maxFlappingDuration  time.Duration
}

// NewFlappingDetector creates a new FlappingDetector with configurable thresholds.
func NewFlappingDetector(maxTransitionsPerDay int, maxFlappingDuration time.Duration) *FlappingDetector {
	return &FlappingDetector{
		maxTransitionsPerDay: maxTransitionsPerDay,
		maxFlappingDuration:  maxFlappingDuration,
	}
}

// IsFlapping returns true if the alert's transition history indicates flapping.
// An alert is flapping if:
// - It has > maxTransitionsPerDay transitions in any 24h window
// - It alternates firing/resolved for longer than maxFlappingDuration
func (d *FlappingDetector) IsFlapping(transitions []StateTransition) bool {
	if len(transitions) == 0 {
		return false
	}

	// Check 1: Too many transitions in any 24h window
	if d.hasExcessiveTransitions(transitions) {
		return true
	}

	// Check 2: Prolonged alternation between firing and normal
	if d.hasProlongedFlapping(transitions) {
		return true
	}

	return false
}

// hasExcessiveTransitions checks if any 24h window has too many transitions.
func (d *FlappingDetector) hasExcessiveTransitions(transitions []StateTransition) bool {
	if len(transitions) <= d.maxTransitionsPerDay {
		return false
	}

	// Sliding window check for any 24h period
	windowSize := 24 * time.Hour

	for i := 0; i < len(transitions); i++ {
		windowStart := transitions[i].Timestamp
		windowEnd := windowStart.Add(windowSize)
		count := 0

		for j := i; j < len(transitions); j++ {
			if transitions[j].Timestamp.Before(windowEnd) || transitions[j].Timestamp.Equal(windowEnd) {
				count++
			} else {
				break
			}
		}

		if count > d.maxTransitionsPerDay {
			return true
		}
	}

	return false
}

// hasProlongedFlapping checks if the alert alternates states for too long.
func (d *FlappingDetector) hasProlongedFlapping(transitions []StateTransition) bool {
	if len(transitions) < 4 {
		return false
	}

	// Look for prolonged firing->normal->firing->normal patterns
	alternationStart := time.Time{}
	alternationCount := 0
	lastToState := ""

	for _, t := range transitions {
		// Track firing<->normal alternations
		isAlternation := (lastToState == "firing" && t.ToState == "normal") ||
			(lastToState == "normal" && t.ToState == "firing")

		if isAlternation {
			if alternationStart.IsZero() {
				alternationStart = t.Timestamp
			}
			alternationCount++

			// Check if this alternation period exceeds max duration
			if alternationCount >= 4 {
				duration := t.Timestamp.Sub(alternationStart)
				if duration > d.maxFlappingDuration {
					return true
				}
			}
		} else {
			// Reset on non-alternation
			alternationStart = time.Time{}
			alternationCount = 0
		}

		lastToState = t.ToState
	}

	return false
}

// FilterFlapping removes flapping alerts from a transition list.
// Returns non-flapping transitions grouped by alert UID.
func (d *FlappingDetector) FilterFlapping(alertTransitions map[string][]StateTransition) map[string][]StateTransition {
	result := make(map[string][]StateTransition)

	for alertUID, transitions := range alertTransitions {
		if !d.IsFlapping(transitions) {
			result[alertUID] = transitions
		}
	}

	return result
}

// IsTransitionSignificant returns true if the transition is worth evaluating.
// Only firing->normal and normal->firing transitions are significant.
func (d *FlappingDetector) IsTransitionSignificant(t StateTransition) bool {
	// We care about transitions to/from firing state
	return (t.FromState == "normal" && t.ToState == "firing") ||
		(t.FromState == "firing" && t.ToState == "normal") ||
		(t.FromState == "pending" && t.ToState == "firing")
}
