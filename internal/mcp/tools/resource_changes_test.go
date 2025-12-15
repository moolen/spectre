package tools

import (
	"testing"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/storage"
)

func TestCalculateImpactScore_SingleErrorEvent(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   1,
		WarningEvents: 0,
		EventCount:    1,
	}

	score := calculateImpactScore(&summary)

	expected := 0.3
	if score != expected {
		t.Errorf("Expected impact score %f, got %f", expected, score)
	}
}

func TestCalculateImpactScore_SingleWarningEvent(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 1,
		EventCount:    1,
	}

	score := calculateImpactScore(&summary)

	expected := 0.15
	if score != expected {
		t.Errorf("Expected impact score %f, got %f", expected, score)
	}
}

func TestCalculateImpactScore_ErrorAndWarning(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   1,
		WarningEvents: 1,
		EventCount:    2,
	}

	score := calculateImpactScore(&summary)

	expected := 0.45 // 0.3 + 0.15
	const epsilon = 0.0001
	if score < expected-epsilon || score > expected+epsilon {
		t.Errorf("Expected impact score %f, got %f", expected, score)
	}
}

func TestCalculateImpactScore_StatusTransitionToError(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		StatusTransitions: []StatusTransition{
			{
				FromStatus: "Ready",
				ToStatus:   "Error",
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.3
	if score != expected {
		t.Errorf("Expected impact score %f for Error transition, got %f", expected, score)
	}
}

func TestCalculateImpactScore_StatusTransitionToWarning(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		StatusTransitions: []StatusTransition{
			{
				FromStatus: "Ready",
				ToStatus:   "Warning",
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.15
	if score != expected {
		t.Errorf("Expected impact score %f for Warning transition, got %f", expected, score)
	}
}

func TestCalculateImpactScore_MultipleErrorTransitions(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		StatusTransitions: []StatusTransition{
			{FromStatus: "Ready", ToStatus: "Error"},
			{FromStatus: "Error", ToStatus: "Ready"},
			{FromStatus: "Ready", ToStatus: "Error"},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.6 // 0.3 + 0.3 (two transitions to Error)
	if score != expected {
		t.Errorf("Expected impact score %f for multiple Error transitions, got %f", expected, score)
	}
}

func TestCalculateImpactScore_HighEventCount(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventCount:    15,
	}

	score := calculateImpactScore(&summary)

	expected := 0.1 // High event count bonus
	if score != expected {
		t.Errorf("Expected impact score %f for high event count, got %f", expected, score)
	}
}

func TestCalculateImpactScore_VeryHighEventCount(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventCount:    55,
	}

	score := calculateImpactScore(&summary)

	expected := 0.3 // 0.1 for >10 + 0.2 for >50
	const epsilon = 0.0001
	if score < expected-epsilon || score > expected+epsilon {
		t.Errorf("Expected impact score %f for very high event count, got %f", expected, score)
	}
}

func TestCalculateImpactScore_CombinedFactors(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   1,
		WarningEvents: 1,
		StatusTransitions: []StatusTransition{
			{FromStatus: "Ready", ToStatus: "Error"},
		},
		EventCount: 15,
	}

	score := calculateImpactScore(&summary)

	// 0.3 (error) + 0.15 (warning) + 0.3 (transition) + 0.1 (high events) = 0.85
	expected := 0.85
	if score != expected {
		t.Errorf("Expected impact score %f for combined factors, got %f", expected, score)
	}
}

func TestCalculateImpactScore_CappedAt1(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   5,
		WarningEvents: 5,
		StatusTransitions: []StatusTransition{
			{FromStatus: "Ready", ToStatus: "Error"},
			{FromStatus: "Ready", ToStatus: "Error"},
			{FromStatus: "Ready", ToStatus: "Error"},
		},
		EventCount: 100,
	}

	score := calculateImpactScore(&summary)

	if score != 1.0 {
		t.Errorf("Expected impact score capped at 1.0, got %f", score)
	}
}

func TestCalculateImpactScore_OOMKilledContainer(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		ContainerIssues: []analyzer.ContainerIssue{
			{
				IssueType:    "OOMKilled",
				ImpactScore:  0.4,
				RestartCount: 3,
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.4 // OOMKilled impact
	if score != expected {
		t.Errorf("Expected impact score %f for OOMKilled, got %f", expected, score)
	}
}

func TestCalculateImpactScore_CrashLoopBackOff(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		ContainerIssues: []analyzer.ContainerIssue{
			{
				IssueType:    "CrashLoopBackOff",
				ImpactScore:  0.35,
				RestartCount: 5,
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.35 // CrashLoopBackOff impact
	if score != expected {
		t.Errorf("Expected impact score %f for CrashLoopBackOff, got %f", expected, score)
	}
}

func TestCalculateImpactScore_ImagePullBackOff(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		ContainerIssues: []analyzer.ContainerIssue{
			{
				IssueType:   "ImagePullBackOff",
				ImpactScore: 0.25,
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.25 // ImagePullBackOff impact
	if score != expected {
		t.Errorf("Expected impact score %f for ImagePullBackOff, got %f", expected, score)
	}
}

func TestCalculateImpactScore_HighRestartCount(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		ContainerIssues: []analyzer.ContainerIssue{
			{
				IssueType:    "HighRestartCount",
				RestartCount: 15,
			},
		},
	}

	score := calculateImpactScore(&summary)

	// 0.2 for HighRestartCount
	expected := 0.2
	if score != expected {
		t.Errorf("Expected impact score %f for high restart count, got %f", expected, score)
	}
}

func TestCalculateImpactScore_SchedulingIssue(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventPatterns: []storage.EventPattern{
			{
				PatternType: "FailedScheduling",
				Reason:      "FailedScheduling",
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.3 // FailedScheduling impact
	if score != expected {
		t.Errorf("Expected impact score %f for FailedScheduling, got %f", expected, score)
	}
}

func TestCalculateImpactScore_EvictionEvent(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventPatterns: []storage.EventPattern{
			{
				PatternType: "Evicted",
				Reason:      "Evicted",
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.35 // Evicted impact
	if score != expected {
		t.Errorf("Expected impact score %f for Evicted, got %f", expected, score)
	}
}

func TestCalculateImpactScore_PreemptionEvent(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventPatterns: []storage.EventPattern{
			{
				PatternType: "Preempted",
				Reason:      "Preempted",
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.25 // Preempted impact
	if score != expected {
		t.Errorf("Expected impact score %f for Preempted, got %f", expected, score)
	}
}

func TestCalculateImpactScore_ProbeFailures(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventPatterns: []storage.EventPattern{
			{
				PatternType: "ProbeFailure",
				Reason:      "Unhealthy",
				Message:     "Probe failed",
			},
		},
	}

	score := calculateImpactScore(&summary)

	expected := 0.25 // ProbeFailure pattern impact
	if score != expected {
		t.Errorf("Expected impact score %f for probe failures, got %f", expected, score)
	}
}

func TestCalculateImpactScore_ComplexScenario(t *testing.T) {
	// Realistic scenario: OOMKilled pod with multiple restarts and events
	summary := ResourceChangeSummary{
		ErrorEvents:   2,
		WarningEvents: 1,
		StatusTransitions: []StatusTransition{
			{FromStatus: "Ready", ToStatus: "Error"},
		},
		ContainerIssues: []analyzer.ContainerIssue{
			{
				IssueType:    "OOMKilled",
				ImpactScore:  0.4,
				RestartCount: 12,
			},
		},
		EventCount: 25,
	}

	score := calculateImpactScore(&summary)

	// Should be capped at 1.0:
	// 0.3 (error) + 0.15 (warning) + 0.3 (transition) + 0.4 (OOMKilled) + 0.2 (restarts) + 0.1 (events) = 1.45 -> 1.0
	if score != 1.0 {
		t.Errorf("Expected impact score capped at 1.0 for complex scenario, got %f", score)
	}
}

func TestCalculateImpactScore_ZeroScore(t *testing.T) {
	summary := ResourceChangeSummary{
		ErrorEvents:   0,
		WarningEvents: 0,
		EventCount:    1, // Low event count
	}

	score := calculateImpactScore(&summary)

	if score != 0.0 {
		t.Errorf("Expected impact score 0.0 for minimal changes, got %f", score)
	}
}

func TestCalculateImpactScore_AllFactorsCombined(t *testing.T) {
	tests := []struct {
		name     string
		summary  ResourceChangeSummary
		expected float64
	}{
		{
			name: "only_error_events",
			summary: ResourceChangeSummary{
				ErrorEvents: 1,
			},
			expected: 0.3,
		},
		{
			name: "only_warning_events",
			summary: ResourceChangeSummary{
				WarningEvents: 1,
			},
			expected: 0.15,
		},
		{
			name: "error_and_warning",
			summary: ResourceChangeSummary{
				ErrorEvents:   1,
				WarningEvents: 1,
			},
			expected: 0.45,
		},
		{
			name: "transition_to_error",
			summary: ResourceChangeSummary{
				StatusTransitions: []StatusTransition{
					{ToStatus: "Error"},
				},
			},
			expected: 0.3,
		},
		{
			name: "high_event_count",
			summary: ResourceChangeSummary{
				EventCount: 15,
			},
			expected: 0.1,
		},
		{
			name: "very_high_event_count",
			summary: ResourceChangeSummary{
				EventCount: 60,
			},
			expected: 0.3, // 0.1 for >10 + 0.2 for >50
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateImpactScore(&tt.summary)
			// Use tolerance for floating point comparison
			const epsilon = 0.0001
			if score < tt.expected-epsilon || score > tt.expected+epsilon {
				t.Errorf("Expected %f, got %f", tt.expected, score)
			}
		})
	}
}
