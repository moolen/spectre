package grafana

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFlappingDetector_IsFlapping(t *testing.T) {
	detector := NewFlappingDetector(50, 2*time.Hour)

	testCases := []struct {
		name        string
		transitions []StateTransition
		expected    bool
	}{
		{
			name:        "empty transitions",
			transitions: []StateTransition{},
			expected:    false,
		},
		{
			name: "stable alert - single transition",
			transitions: []StateTransition{
				{FromState: "normal", ToState: "firing", Timestamp: time.Now()},
			},
			expected: false,
		},
		{
			name: "excessive transitions in 24h window",
			transitions: func() []StateTransition {
				var transitions []StateTransition
				baseTime := time.Now()
				for i := 0; i < 60; i++ {
					from := "normal"
					to := "firing"
					if i%2 == 1 {
						from, to = to, from
					}
					transitions = append(transitions, StateTransition{
						FromState: from,
						ToState:   to,
						Timestamp: baseTime.Add(time.Duration(i) * 20 * time.Minute),
					})
				}
				return transitions
			}(),
			expected: true,
		},
		{
			name: "moderate transitions - not flapping",
			transitions: func() []StateTransition {
				baseTime := time.Now()
				// 4 transitions over several days - clearly not flapping
				return []StateTransition{
					{FromState: "normal", ToState: "firing", Timestamp: baseTime},
					{FromState: "firing", ToState: "normal", Timestamp: baseTime.Add(6 * time.Hour)},
					{FromState: "normal", ToState: "firing", Timestamp: baseTime.Add(24 * time.Hour)},
					{FromState: "firing", ToState: "normal", Timestamp: baseTime.Add(30 * time.Hour)},
				}
			}(),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.IsFlapping(tc.transitions)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFlappingDetector_IsTransitionSignificant(t *testing.T) {
	detector := NewFlappingDetector(50, 2*time.Hour)

	testCases := []struct {
		transition StateTransition
		expected   bool
	}{
		{
			transition: StateTransition{FromState: "normal", ToState: "firing"},
			expected:   true,
		},
		{
			transition: StateTransition{FromState: "firing", ToState: "normal"},
			expected:   true,
		},
		{
			transition: StateTransition{FromState: "pending", ToState: "firing"},
			expected:   true,
		},
		{
			transition: StateTransition{FromState: "normal", ToState: "pending"},
			expected:   false,
		},
		{
			transition: StateTransition{FromState: "pending", ToState: "normal"},
			expected:   false,
		},
	}

	for _, tc := range testCases {
		name := tc.transition.FromState + " -> " + tc.transition.ToState
		t.Run(name, func(t *testing.T) {
			result := detector.IsTransitionSignificant(tc.transition)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFlappingDetector_FilterFlapping(t *testing.T) {
	detector := NewFlappingDetector(5, 2*time.Hour)

	baseTime := time.Now()

	alertTransitions := map[string][]StateTransition{
		"stable-alert": {
			{FromState: "normal", ToState: "firing", Timestamp: baseTime},
			{FromState: "firing", ToState: "normal", Timestamp: baseTime.Add(time.Hour)},
		},
		"flapping-alert": func() []StateTransition {
			var transitions []StateTransition
			for i := 0; i < 10; i++ {
				from := "normal"
				to := "firing"
				if i%2 == 1 {
					from, to = to, from
				}
				transitions = append(transitions, StateTransition{
					FromState: from,
					ToState:   to,
					Timestamp: baseTime.Add(time.Duration(i) * 2 * time.Hour),
				})
			}
			return transitions
		}(),
	}

	result := detector.FilterFlapping(alertTransitions)

	assert.Contains(t, result, "stable-alert")
	assert.NotContains(t, result, "flapping-alert")
}
