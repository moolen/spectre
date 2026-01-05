package analysis

import (
	"testing"
	"time"
)

func TestCalculateChangeEventSignificance(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		event           *ChangeEventInfo
		isOnCausalSpine bool
		failureTime     time.Time
		errorPatterns   []string
		minScore        float64
		maxScore        float64
		expectReasons   []string
	}{
		{
			name: "config change on causal spine within 5 minutes",
			event: &ChangeEventInfo{
				EventID:       "1",
				Timestamp:     now.Add(-2 * time.Minute),
				EventType:     "UPDATE",
				ConfigChanged: true,
			},
			isOnCausalSpine: true,
			failureTime:     now,
			errorPatterns:   nil,
			minScore:        0.7,
			maxScore:        1.0,
			expectReasons:   []string{"on causal path", "spec changed", "within 5min of failure"},
		},
		{
			name: "status change only",
			event: &ChangeEventInfo{
				EventID:       "2",
				Timestamp:     now.Add(-10 * time.Minute),
				EventType:     "UPDATE",
				StatusChanged: true,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   nil,
			minScore:        0.05,
			maxScore:        0.3,
			expectReasons:   []string{"status changed"},
		},
		{
			name: "DELETE event on spine",
			event: &ChangeEventInfo{
				EventID:   "3",
				Timestamp: now.Add(-1 * time.Minute),
				EventType: "DELETE",
			},
			isOnCausalSpine: true,
			failureTime:     now,
			errorPatterns:   nil,
			minScore:        0.3,
			maxScore:        0.5,
			expectReasons:   []string{"on causal path", "within 5min of failure", "resource deleted"},
		},
		{
			name: "CREATE event",
			event: &ChangeEventInfo{
				EventID:   "4",
				Timestamp: now.Add(-3 * time.Minute),
				EventType: "CREATE",
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   nil,
			minScore:        0.2,
			maxScore:        0.4,
			expectReasons:   []string{"within 5min of failure", "resource created"},
		},
		{
			name: "error pattern match",
			event: &ChangeEventInfo{
				EventID:     "5",
				Timestamp:   now.Add(-4 * time.Minute),
				EventType:   "UPDATE",
				Description: "ImagePullBackOff: failed to pull image",
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   []string{"image", "pull"},
			minScore:        0.3,
			maxScore:        0.6,
			expectReasons:   []string{"within 5min of failure", "matches error pattern"},
		},
		{
			name: "event within 30 minutes",
			event: &ChangeEventInfo{
				EventID:       "6",
				Timestamp:     now.Add(-20 * time.Minute),
				EventType:     "UPDATE",
				ConfigChanged: true,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   nil,
			minScore:        0.4,
			maxScore:        0.6,
			expectReasons:   []string{"spec changed", "within 30min of failure"},
		},
		{
			name: "event older than 1 hour",
			event: &ChangeEventInfo{
				EventID:   "7",
				Timestamp: now.Add(-2 * time.Hour),
				EventType: "UPDATE",
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   nil,
			minScore:        0.0,
			maxScore:        0.1,
			expectReasons:   []string{},
		},
		{
			name: "all factors combined - high score",
			event: &ChangeEventInfo{
				EventID:       "8",
				Timestamp:     now.Add(-1 * time.Minute),
				EventType:     "DELETE",
				ConfigChanged: true,
				Description:   "Pod deleted due to OOM killed",
			},
			isOnCausalSpine: true,
			failureTime:     now,
			errorPatterns:   []string{"oom", "killed"},
			minScore:        0.9,
			maxScore:        1.0,
			expectReasons:   []string{"on causal path", "spec changed", "within 5min of failure", "matches error pattern", "resource deleted"},
		},
		{
			name: "zero failure time",
			event: &ChangeEventInfo{
				EventID:       "9",
				Timestamp:     now,
				EventType:     "UPDATE",
				ConfigChanged: true,
			},
			isOnCausalSpine: true,
			failureTime:     time.Time{}, // zero time
			errorPatterns:   nil,
			minScore:        0.4,
			maxScore:        0.6,
			expectReasons:   []string{"on causal path", "spec changed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateChangeEventSignificance(
				tt.event,
				tt.isOnCausalSpine,
				tt.failureTime,
				tt.errorPatterns,
			)

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Score < tt.minScore || result.Score > tt.maxScore {
				t.Errorf("score %.2f not in expected range [%.2f, %.2f]", result.Score, tt.minScore, tt.maxScore)
			}

			for _, expectedReason := range tt.expectReasons {
				found := false
				for _, reason := range result.Reasons {
					if reason == expectedReason {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected reason %q not found in %v", expectedReason, result.Reasons)
				}
			}
		})
	}
}

func TestCalculateK8sEventSignificance(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		event           *K8sEventInfo
		isOnCausalSpine bool
		failureTime     time.Time
		minScore        float64
		maxScore        float64
		expectReasons   []string
	}{
		{
			name: "Warning event with CrashLoopBackOff",
			event: &K8sEventInfo{
				EventID:   "1",
				Timestamp: now.Add(-2 * time.Minute),
				Reason:    "CrashLoopBackOff",
				Type:      "Warning",
				Count:     3,
			},
			isOnCausalSpine: true,
			failureTime:     now,
			minScore:        0.9,
			maxScore:        1.0,
			expectReasons:   []string{"warning event", "CrashLoopBackOff", "on causal path"},
		},
		{
			name: "Warning event with ImagePullBackOff",
			event: &K8sEventInfo{
				EventID:   "2",
				Timestamp: now.Add(-1 * time.Minute),
				Reason:    "ImagePullBackOff",
				Type:      "Warning",
				Count:     5,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			minScore:        0.8,
			maxScore:        1.0,
			expectReasons:   []string{"warning event", "ImagePullBackOff"},
		},
		{
			name: "Error event with OOMKilled",
			event: &K8sEventInfo{
				EventID:   "3",
				Timestamp: now.Add(-30 * time.Second),
				Reason:    "OOMKilled",
				Type:      "Error",
				Count:     1,
			},
			isOnCausalSpine: true,
			failureTime:     now,
			minScore:        0.9,
			maxScore:        1.0,
			expectReasons:   []string{"error event", "OOMKilled", "on causal path"},
		},
		{
			name: "Normal event with Scheduled",
			event: &K8sEventInfo{
				EventID:   "4",
				Timestamp: now.Add(-5 * time.Minute),
				Reason:    "Scheduled",
				Type:      "Normal",
				Count:     1,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			minScore:        0.1,
			maxScore:        0.3,
			expectReasons:   []string{"Scheduled"},
		},
		{
			name: "High count event",
			event: &K8sEventInfo{
				EventID:   "5",
				Timestamp: now.Add(-10 * time.Minute),
				Reason:    "BackOff",
				Type:      "Warning",
				Count:     10,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			minScore:        0.7,
			maxScore:        1.0,
			expectReasons:   []string{"warning event", "BackOff", "repeated event"},
		},
		{
			name: "FailedScheduling event",
			event: &K8sEventInfo{
				EventID:   "6",
				Timestamp: now.Add(-3 * time.Minute),
				Reason:    "FailedScheduling",
				Type:      "Warning",
				Count:     2,
			},
			isOnCausalSpine: true,
			failureTime:     now,
			minScore:        0.9,
			maxScore:        1.0,
			expectReasons:   []string{"warning event", "FailedScheduling", "on causal path"},
		},
		{
			name: "Unknown reason",
			event: &K8sEventInfo{
				EventID:   "7",
				Timestamp: now.Add(-5 * time.Minute),
				Reason:    "UnknownReason",
				Type:      "Normal",
				Count:     1,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			minScore:        0.0,
			maxScore:        0.2,
			expectReasons:   []string{},
		},
		{
			name: "Event within 30 minutes",
			event: &K8sEventInfo{
				EventID:   "8",
				Timestamp: now.Add(-20 * time.Minute),
				Reason:    "Unhealthy",
				Type:      "Warning",
				Count:     3,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			minScore:        0.7,
			maxScore:        0.9,
			expectReasons:   []string{"warning event", "Unhealthy"},
		},
		{
			name: "FailedMount event",
			event: &K8sEventInfo{
				EventID:   "9",
				Timestamp: now.Add(-2 * time.Minute),
				Reason:    "FailedMount",
				Type:      "Warning",
				Count:     2,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			minScore:        0.7,
			maxScore:        0.9,
			expectReasons:   []string{"warning event", "FailedMount"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateK8sEventSignificance(
				tt.event,
				tt.isOnCausalSpine,
				tt.failureTime,
			)

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Score < tt.minScore || result.Score > tt.maxScore {
				t.Errorf("score %.2f not in expected range [%.2f, %.2f]", result.Score, tt.minScore, tt.maxScore)
			}

			for _, expectedReason := range tt.expectReasons {
				found := false
				for _, reason := range result.Reasons {
					if reason == expectedReason {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected reason %q not found in %v", expectedReason, result.Reasons)
				}
			}
		})
	}
}

func TestExtractErrorPatterns(t *testing.T) {
	tests := []struct {
		name          string
		errorMessage  string
		expectPattern []string
		expectNil     bool
	}{
		{
			name:          "empty message",
			errorMessage:  "",
			expectPattern: nil,
			expectNil:     true,
		},
		{
			name:          "image pull error",
			errorMessage:  "Failed to pull image: unauthorized",
			expectPattern: []string{"image", "pull"},
		},
		{
			name:          "OOM killed",
			errorMessage:  "Container was OOM killed due to memory pressure",
			expectPattern: []string{"memory", "oom", "killed"},
		},
		{
			name:          "config error",
			errorMessage:  "ConfigMap not found: app-config",
			expectPattern: []string{"config", "configmap"},
		},
		{
			name:          "volume mount error",
			errorMessage:  "Failed to mount volume: permission denied",
			expectPattern: []string{"volume", "mount", "permission", "denied"},
		},
		{
			name:          "scheduling error",
			errorMessage:  "0/3 nodes are available: node affinity mismatch",
			expectPattern: []string{"node", "affinity"},
		},
		{
			name:          "connection error",
			errorMessage:  "Connection refused to service endpoint",
			expectPattern: []string{"connection", "refused"},
		},
		{
			name:          "probe failure",
			errorMessage:  "Liveness probe failed: unhealthy",
			expectPattern: []string{"liveness", "probe", "unhealthy"},
		},
		{
			name:          "crash loop",
			errorMessage:  "Back-off restarting crashed container",
			expectPattern: []string{"crash", "restart"},
		},
		{
			name:          "secret error",
			errorMessage:  "Secret not found: api-credentials",
			expectPattern: []string{"secret"},
		},
		{
			name:          "timeout error",
			errorMessage:  "Request timeout after 30s",
			expectPattern: []string{"timeout"},
		},
		{
			name:          "case insensitive",
			errorMessage:  "IMAGE PULL failed, CONFIG error",
			expectPattern: []string{"image", "pull", "config"},
		},
		{
			name:          "no matching patterns",
			errorMessage:  "Unknown error occurred",
			expectPattern: []string{},
			expectNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractErrorPatterns(tt.errorMessage)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			for _, expected := range tt.expectPattern {
				found := false
				for _, pattern := range result {
					if pattern == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected pattern %q not found in %v", expected, result)
				}
			}
		})
	}
}

func TestScoreEvents(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		node            *GraphNode
		isOnCausalSpine bool
		failureTime     time.Time
		errorPatterns   []string
	}{
		{
			name: "scores all event types",
			node: &GraphNode{
				ID: "test-node",
				ChangeEvent: &ChangeEventInfo{
					EventID:       "change-1",
					Timestamp:     now.Add(-2 * time.Minute),
					EventType:     "UPDATE",
					ConfigChanged: true,
				},
				AllEvents: []ChangeEventInfo{
					{
						EventID:       "change-2",
						Timestamp:     now.Add(-5 * time.Minute),
						EventType:     "CREATE",
						StatusChanged: true,
					},
				},
				K8sEvents: []K8sEventInfo{
					{
						EventID:   "k8s-1",
						Timestamp: now.Add(-1 * time.Minute),
						Reason:    "BackOff",
						Type:      "Warning",
						Count:     3,
					},
				},
			},
			isOnCausalSpine: true,
			failureTime:     now,
			errorPatterns:   []string{"error"},
		},
		{
			name: "handles nil ChangeEvent",
			node: &GraphNode{
				ID:          "test-node",
				ChangeEvent: nil,
				AllEvents: []ChangeEventInfo{
					{
						EventID:   "change-1",
						Timestamp: now,
						EventType: "UPDATE",
					},
				},
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   nil,
		},
		{
			name: "handles empty events",
			node: &GraphNode{
				ID:          "test-node",
				ChangeEvent: nil,
				AllEvents:   nil,
				K8sEvents:   nil,
			},
			isOnCausalSpine: false,
			failureTime:     now,
			errorPatterns:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			ScoreEvents(tt.node, tt.isOnCausalSpine, tt.failureTime, tt.errorPatterns)

			// Verify significance is set
			if tt.node.ChangeEvent != nil {
				if tt.node.ChangeEvent.Significance == nil {
					t.Error("expected ChangeEvent.Significance to be set")
				}
			}

			for i, evt := range tt.node.AllEvents {
				if evt.Significance == nil {
					t.Errorf("expected AllEvents[%d].Significance to be set", i)
				}
			}

			for i, evt := range tt.node.K8sEvents {
				if evt.Significance == nil {
					t.Errorf("expected K8sEvents[%d].Significance to be set", i)
				}
			}
		})
	}
}

func TestSignificantK8sEventReasons(t *testing.T) {
	// Test that all expected reasons are in the map with reasonable values
	criticalReasons := []string{
		"FailedScheduling",
		"ImagePullBackOff",
		"CrashLoopBackOff",
		"OOMKilled",
	}

	for _, reason := range criticalReasons {
		boost, ok := SignificantK8sEventReasons[reason]
		if !ok {
			t.Errorf("critical reason %q not found in SignificantK8sEventReasons", reason)
			continue
		}
		if boost < 0.4 {
			t.Errorf("critical reason %q has low boost %.2f, expected >= 0.4", reason, boost)
		}
	}

	// Test that normal events have low boosts
	normalReasons := []string{
		"Pulled",
		"Started",
		"Created",
	}

	for _, reason := range normalReasons {
		boost, ok := SignificantK8sEventReasons[reason]
		if !ok {
			t.Errorf("normal reason %q not found in SignificantK8sEventReasons", reason)
			continue
		}
		if boost > 0.2 {
			t.Errorf("normal reason %q has high boost %.2f, expected <= 0.2", reason, boost)
		}
	}
}

func TestScoreNormalization(t *testing.T) {
	now := time.Now()

	// Create an event that would have very high score without normalization
	event := &ChangeEventInfo{
		EventID:       "1",
		Timestamp:     now.Add(-30 * time.Second),
		EventType:     "DELETE",
		ConfigChanged: true,
		StatusChanged: true,
		Description:   "Failed to pull image, config error, memory OOM",
	}

	result := CalculateChangeEventSignificance(
		event,
		true, // on causal spine
		now,
		[]string{"image", "config", "memory", "oom"},
	)

	if result.Score > 1.0 {
		t.Errorf("score %.2f exceeds 1.0, normalization failed", result.Score)
	}
	if result.Score < 0.0 {
		t.Errorf("score %.2f is negative, normalization failed", result.Score)
	}
}

func TestK8sScoreNormalization(t *testing.T) {
	now := time.Now()

	// Create a K8s event that would have very high score without normalization
	event := &K8sEventInfo{
		EventID:   "1",
		Timestamp: now.Add(-30 * time.Second),
		Reason:    "CrashLoopBackOff",
		Type:      "Error",
		Count:     100,
	}

	result := CalculateK8sEventSignificance(event, true, now)

	if result.Score > 1.0 {
		t.Errorf("score %.2f exceeds 1.0, normalization failed", result.Score)
	}
	if result.Score < 0.0 {
		t.Errorf("score %.2f is negative, normalization failed", result.Score)
	}
}
