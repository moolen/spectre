package analysis

import (
	"testing"
	"time"
)

func TestDetectPodPatterns_ContainerOOMKilled(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.containerStatuses.0.state.terminated.exitCode",
				NewValue: float64(137),
				Op:       "add",
			},
		},
	}

	patterns := detectPodPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect OOMKilled pattern")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternContainerOOMKilled {
			found = true
			if p.Severity != 0.50 {
				t.Errorf("Expected severity 0.50, got %.2f", p.Severity)
			}
			if p.Path != "status.containerStatuses.0.state.terminated.exitCode" {
				t.Errorf("Expected specific path, got %s", p.Path)
			}
		}
	}

	if !found {
		t.Error("Did not find OOMKilled pattern")
	}
}

func TestDetectPodPatterns_ContainerCrashed(t *testing.T) {
	tests := []struct {
		name           string
		exitCode       int32
		expectedType   string
		expectedMinSev float64
	}{
		{
			name:           "General error exit code 1",
			exitCode:       1,
			expectedType:   PatternContainerCrashed,
			expectedMinSev: 0.40,
		},
		{
			name:           "Segmentation fault exit code 139",
			exitCode:       139,
			expectedType:   PatternContainerCrashed,
			expectedMinSev: 0.45,
		},
		{
			name:           "SIGTERM exit code 143",
			exitCode:       143,
			expectedType:   PatternContainerCrashed,
			expectedMinSev: 0.40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &ChangeEventInfo{
				EventID:   "event-1",
				Timestamp: time.Now(),
				EventType: "UPDATE",
				Diff: []EventDiff{
					{
						Path:     "status.containerStatuses.0.state.terminated.exitCode",
						NewValue: float64(tt.exitCode),
						Op:       "replace",
					},
				},
			}

			patterns := detectPodPatterns(event)

			if len(patterns) == 0 {
				t.Fatalf("Expected to detect crash pattern for exit code %d", tt.exitCode)
			}

			found := false
			for _, p := range patterns {
				if p.Type == tt.expectedType {
					found = true
					if p.Severity < tt.expectedMinSev {
						t.Errorf("Expected severity >= %.2f, got %.2f", tt.expectedMinSev, p.Severity)
					}
				}
			}

			if !found {
				t.Errorf("Did not find expected pattern type: %s", tt.expectedType)
			}
		})
	}
}

func TestDetectPodPatterns_CrashLoopBackOff(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.containerStatuses.0.state.waiting.reason",
				NewValue: "CrashLoopBackOff",
				Op:       "replace",
			},
		},
	}

	patterns := detectPodPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect CrashLoopBackOff pattern")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternContainerCrashLoopBackOff {
			found = true
			if p.Severity != 0.50 {
				t.Errorf("Expected severity 0.50, got %.2f", p.Severity)
			}
		}
	}

	if !found {
		t.Error("Did not find CrashLoopBackOff pattern")
	}
}

func TestDetectPodPatterns_ImagePullFailed(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{
			name:   "ImagePullBackOff",
			reason: "ImagePullBackOff",
		},
		{
			name:   "ErrImagePull",
			reason: "ErrImagePull",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &ChangeEventInfo{
				EventID:   "event-1",
				Timestamp: time.Now(),
				EventType: "UPDATE",
				Diff: []EventDiff{
					{
						Path:     "status.containerStatuses.0.state.waiting.reason",
						NewValue: tt.reason,
						Op:       "add",
					},
				},
			}

			patterns := detectPodPatterns(event)

			if len(patterns) == 0 {
				t.Fatalf("Expected to detect ImagePullFailed pattern for reason: %s", tt.reason)
			}

			found := false
			for _, p := range patterns {
				if p.Type == PatternContainerImagePullFailed {
					found = true
					if p.Severity != 0.45 {
						t.Errorf("Expected severity 0.45, got %.2f", p.Severity)
					}
				}
			}

			if !found {
				t.Error("Did not find ImagePullFailed pattern")
			}
		})
	}
}

func TestDetectPodPatterns_InitContainer(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.initContainerStatuses.0.state.terminated.exitCode",
				NewValue: float64(1),
				Op:       "add",
			},
		},
	}

	patterns := detectPodPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect init container failure")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternContainerCrashed {
			found = true
		}
	}

	if !found {
		t.Error("Did not detect init container crash")
	}
}

func TestDetectPodPatterns_ContainerTerminatedByReason(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.containerStatuses.0.state.terminated.reason",
				NewValue: "OOMKilled",
				Op:       "add",
			},
		},
	}

	patterns := detectPodPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect OOMKilled by reason")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternContainerOOMKilled {
			found = true
		}
	}

	if !found {
		t.Error("Did not detect OOMKilled pattern from reason field")
	}
}

func TestDetectPodPatterns_ContainerStoppedRunning(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.containerStatuses.0.state.running",
				OldValue: map[string]any{"startedAt": "2024-01-01T00:00:00Z"},
				Op:       "remove",
			},
		},
	}

	patterns := detectPodPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect container stopped running")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternContainerTerminated {
			found = true
			if p.Severity != 0.30 {
				t.Errorf("Expected severity 0.30, got %.2f", p.Severity)
			}
		}
	}

	if !found {
		t.Error("Did not detect container termination")
	}
}

func TestDetectPodPatterns_ProbeFailures(t *testing.T) {
	tests := []struct {
		name         string
		reason       string
		expectedType string
		severity     float64
	}{
		{
			name:         "Liveness probe failure",
			reason:       "LivenessProbe failed",
			expectedType: PatternLivenessProbeFailure,
			severity:     0.40,
		},
		{
			name:         "Readiness probe failure",
			reason:       "ReadinessProbe failed",
			expectedType: PatternReadinessProbeFailure,
			severity:     0.30,
		},
		{
			name:         "Startup probe failure",
			reason:       "StartupProbe failed",
			expectedType: PatternStartupProbeFailure,
			severity:     0.35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &ChangeEventInfo{
				EventID:   "event-1",
				Timestamp: time.Now(),
				EventType: "UPDATE",
				Diff: []EventDiff{
					{
						Path:     "status.conditions.0.reason",
						NewValue: tt.reason,
						Op:       "replace",
					},
				},
			}

			patterns := detectPodPatterns(event)

			if len(patterns) == 0 {
				t.Fatalf("Expected to detect probe failure: %s", tt.name)
			}

			found := false
			for _, p := range patterns {
				if p.Type == tt.expectedType {
					found = true
					if p.Severity != tt.severity {
						t.Errorf("Expected severity %.2f, got %.2f", tt.severity, p.Severity)
					}
				}
			}

			if !found {
				t.Errorf("Did not find expected pattern type: %s", tt.expectedType)
			}
		})
	}
}

func TestDetectPodPatterns_PodEvicted(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		FullSnapshot: map[string]any{
			"status": map[string]any{
				"phase":  "Failed",
				"reason": "Evicted",
			},
		},
	}

	patterns := detectPodPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect pod eviction")
	}

	foundEvicted := false
	foundFailed := false

	for _, p := range patterns {
		if p.Type == PatternPodEvicted {
			foundEvicted = true
			if p.Severity != 0.45 {
				t.Errorf("Expected eviction severity 0.45, got %.2f", p.Severity)
			}
		}
		if p.Type == PatternContainerCrashed && p.Path == "status.phase" {
			foundFailed = true
		}
	}

	if !foundEvicted {
		t.Error("Did not detect pod eviction")
	}
	if !foundFailed {
		t.Error("Did not detect failed phase")
	}
}

func TestDetectDeploymentPatterns_UnavailableReplicas(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.unavailableReplicas",
				OldValue: float64(0),
				NewValue: float64(3),
				Op:       "replace",
			},
		},
	}

	patterns := detectDeploymentPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect unavailable replicas")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternReplicasUnavailable {
			found = true
			if p.Severity != 0.35 {
				t.Errorf("Expected severity 0.35, got %.2f", p.Severity)
			}
		}
	}

	if !found {
		t.Error("Did not detect unavailable replicas pattern")
	}
}

func TestDetectDeploymentPatterns_RolloutStalled(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.conditions.0.reason",
				NewValue: "ProgressDeadlineExceeded",
				Op:       "replace",
			},
		},
	}

	patterns := detectDeploymentPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect rollout stall")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternRolloutStalled {
			found = true
			if p.Severity != 0.40 {
				t.Errorf("Expected severity 0.40, got %.2f", p.Severity)
			}
		}
	}

	if !found {
		t.Error("Did not detect rollout stalled pattern")
	}
}

func TestDetectReplicaSetPatterns(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.readyReplicas",
				OldValue: float64(3),
				NewValue: float64(0),
				Op:       "replace",
			},
		},
	}

	patterns := detectReplicaSetPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect unavailable replicas in ReplicaSet")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternReplicasUnavailable {
			found = true
			if p.Severity != 0.30 {
				t.Errorf("Expected severity 0.30, got %.2f", p.Severity)
			}
		}
	}

	if !found {
		t.Error("Did not detect ReplicaSet unavailable replicas")
	}
}

func TestDetectStatefulSetPatterns(t *testing.T) {
	event := &ChangeEventInfo{
		EventID:   "event-1",
		Timestamp: time.Now(),
		EventType: "UPDATE",
		Diff: []EventDiff{
			{
				Path:     "status.readyReplicas",
				OldValue: float64(3),
				NewValue: float64(0),
				Op:       "replace",
			},
		},
	}

	patterns := detectStatefulSetPatterns(event)

	if len(patterns) == 0 {
		t.Fatal("Expected to detect unavailable replicas in StatefulSet")
	}

	found := false
	for _, p := range patterns {
		if p.Type == PatternReplicasUnavailable {
			found = true
			if p.Severity != 0.35 {
				t.Errorf("Expected severity 0.35, got %.2f", p.Severity)
			}
		}
	}

	if !found {
		t.Error("Did not detect StatefulSet unavailable replicas")
	}
}

func TestGetHighestSeverityPattern(t *testing.T) {
	patterns := []DetectedPattern{
		{
			Type:        PatternContainerTerminated,
			Severity:    0.30,
			Description: "Container stopped",
		},
		{
			Type:        PatternContainerOOMKilled,
			Severity:    0.50,
			Description: "OOMKilled",
		},
		{
			Type:        PatternContainerCrashed,
			Severity:    0.40,
			Description: "Crashed",
		},
	}

	highest := GetHighestSeverityPattern(patterns)

	if highest == nil {
		t.Fatal("Expected to get highest severity pattern")
	}

	if highest.Type != PatternContainerOOMKilled {
		t.Errorf("Expected OOMKilled to be highest, got %s", highest.Type)
	}

	if highest.Severity != 0.50 {
		t.Errorf("Expected severity 0.50, got %.2f", highest.Severity)
	}
}

func TestGetHighestSeverityPattern_Empty(t *testing.T) {
	patterns := []DetectedPattern{}

	highest := GetHighestSeverityPattern(patterns)

	if highest != nil {
		t.Error("Expected nil for empty patterns slice")
	}
}

func TestExtractContainerIndexFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Container status path",
			path:     "status.containerStatuses.2.state.terminated.exitCode",
			expected: "2",
		},
		{
			name:     "Init container status path",
			path:     "status.initContainerStatuses.0.state.waiting.reason",
			expected: "0",
		},
		{
			name:     "No container index",
			path:     "status.phase",
			expected: "",
		},
		{
			name:     "Invalid index",
			path:     "status.containerStatuses.abc.state",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractContainerIndexFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDetectResourcePatterns_Integration(t *testing.T) {
	// Test that DetectResourcePatterns correctly dispatches to resource-specific detectors
	tests := []struct {
		name         string
		resourceKind string
		event        *ChangeEventInfo
		expectCount  int
	}{
		{
			name:         "Pod with OOMKill",
			resourceKind: "Pod",
			event: &ChangeEventInfo{
				EventID: "event-1",
				Diff: []EventDiff{
					{
						Path:     "status.containerStatuses.0.state.terminated.exitCode",
						NewValue: float64(137),
						Op:       "add",
					},
				},
			},
			expectCount: 1,
		},
		{
			name:         "Deployment with unavailable replicas",
			resourceKind: "Deployment",
			event: &ChangeEventInfo{
				EventID: "event-1",
				Diff: []EventDiff{
					{
						Path:     "status.unavailableReplicas",
						NewValue: float64(5),
						Op:       "replace",
					},
				},
			},
			expectCount: 1,
		},
		{
			name:         "Unknown resource kind",
			resourceKind: "ConfigMap",
			event: &ChangeEventInfo{
				EventID: "event-1",
				Diff: []EventDiff{
					{
						Path:     "data.key",
						NewValue: "value",
						Op:       "replace",
					},
				},
			},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := DetectResourcePatterns(tt.event, tt.resourceKind)
			if len(patterns) != tt.expectCount {
				t.Errorf("Expected %d patterns, got %d", tt.expectCount, len(patterns))
			}
		})
	}
}

func TestExitCodeMeanings(t *testing.T) {
	// Test that common exit codes have meanings
	expectedCodes := []int32{0, 1, 137, 139, 143}

	for _, code := range expectedCodes {
		meaning := exitCodeMeanings[code]
		if meaning == "" {
			t.Errorf("Exit code %d should have a meaning", code)
		}
	}
}
