package storage

import (
	"strings"
	"testing"
)

func TestDetectSchedulingFailure(t *testing.T) {
	tests := []struct {
		name         string
		event        K8sEventData
		shouldDetect bool
		containsText string // Text that should be present in details
	}{
		{
			name: "InsufficientCPU",
			event: K8sEventData{
				Reason:  "FailedScheduling",
				Message: "0/5 nodes are available: 3 Insufficient cpu, 2 node selector mismatch",
				Type:    "Warning",
			},
			shouldDetect: true,
			containsText: "node selector",
		},
		{
			name: "InsufficientMemory",
			event: K8sEventData{
				Reason:  "FailedScheduling",
				Message: "0/3 nodes are available: 3 Insufficient memory",
				Type:    "Warning",
			},
			shouldDetect: true,
			containsText: "Insufficient memory",
		},
		{
			name: "TaintToleration",
			event: K8sEventData{
				Reason:  "FailedScheduling",
				Message: "0/3 nodes are available: 1 node(s) had taint {key: value}, that the pod didn't tolerate",
				Type:    "Warning",
			},
			shouldDetect: true,
			containsText: "taint",
		},
		{
			name: "NotSchedulingFailure",
			event: K8sEventData{
				Reason:  "Started",
				Message: "Container started",
				Type:    "Normal",
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := detectSchedulingFailure(tt.event)

			if tt.shouldDetect {
				if pattern == nil {
					t.Fatal("Expected to detect scheduling failure, got nil")
				}
				if pattern.PatternType != "FailedScheduling" {
					t.Errorf("Expected PatternType FailedScheduling, got %s", pattern.PatternType)
				}
				if pattern.ImpactScore != 0.30 {
					t.Errorf("Expected impact score 0.30, got %f", pattern.ImpactScore)
				}
				if !strings.Contains(strings.ToLower(pattern.Details), strings.ToLower(tt.containsText)) && tt.containsText != "" {
					t.Errorf("Expected details to contain '%s', got '%s'", tt.containsText, pattern.Details)
				}
			} else if pattern != nil {
				t.Errorf("Expected no detection, got pattern: %+v", pattern)
			}
		})
	}
}

func TestDetectEviction(t *testing.T) {
	tests := []struct {
		name          string
		event         K8sEventData
		shouldDetect  bool
		expectedCause string
	}{
		{
			name: "MemoryPressure",
			event: K8sEventData{
				Reason:  "Evicted",
				Message: "The node was low on resource: memory",
				Type:    "Warning",
			},
			shouldDetect:  true,
			expectedCause: "Memory pressure",
		},
		{
			name: "DiskPressure",
			event: K8sEventData{
				Reason:  "Evicted",
				Message: "The node was low on resource: disk",
				Type:    "Warning",
			},
			shouldDetect:  true,
			expectedCause: "Disk pressure",
		},
		{
			name: "EphemeralStorage",
			event: K8sEventData{
				Reason:  "Evicted",
				Message: "Pod ephemeral local storage usage exceeds the total limit",
				Type:    "Warning",
			},
			shouldDetect:  true,
			expectedCause: "Ephemeral storage pressure",
		},
		{
			name: "NotEviction",
			event: K8sEventData{
				Reason:  "Killing",
				Message: "Stopping container",
				Type:    "Normal",
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := detectEviction(tt.event)

			if tt.shouldDetect {
				if pattern == nil {
					t.Fatal("Expected to detect eviction, got nil")
				}
				if pattern.PatternType != "Evicted" {
					t.Errorf("Expected PatternType Evicted, got %s", pattern.PatternType)
				}
				if pattern.ImpactScore != 0.35 {
					t.Errorf("Expected impact score 0.35, got %f", pattern.ImpactScore)
				}
				if pattern.Details != tt.expectedCause {
					t.Errorf("Expected cause '%s', got '%s'", tt.expectedCause, pattern.Details)
				}
			} else if pattern != nil {
				t.Errorf("Expected no detection, got pattern: %+v", pattern)
			}
		})
	}
}

func TestDetectPreemption(t *testing.T) {
	event := K8sEventData{
		Reason:  "Preempted",
		Message: "Pod was preempted to make room for higher priority pod",
		Type:    "Normal",
	}

	pattern := detectPreemption(event)

	if pattern == nil {
		t.Fatal("Expected to detect preemption, got nil")
	}

	if pattern.PatternType != "Preempted" {
		t.Errorf("Expected PatternType Preempted, got %s", pattern.PatternType)
	}

	if pattern.ImpactScore != 0.25 {
		t.Errorf("Expected impact score 0.25, got %f", pattern.ImpactScore)
	}
}

func TestDetectDNSFailure(t *testing.T) {
	tests := []struct {
		name         string
		event        K8sEventData
		shouldDetect bool
		expectedHost string
	}{
		{
			name: "NoSuchHost",
			event: K8sEventData{
				Reason:  "BackOff",
				Message: "Back-off restarting failed container: dial tcp: lookup api.example.com: no such host",
				Type:    "Warning",
			},
			shouldDetect: true,
			expectedHost: "api.example.com",
		},
		{
			name: "DNSError",
			event: K8sEventData{
				Reason:  "Failed",
				Message: "Error: DNS lookup failed for service.namespace.svc.cluster.local",
				Type:    "Warning",
			},
			shouldDetect: true,
		},
		{
			name: "LookupTimeout",
			event: K8sEventData{
				Reason:  "Failed",
				Message: "dial tcp: lookup database: i/o timeout",
				Type:    "Warning",
			},
			shouldDetect: true,
		},
		{
			name: "NotDNSFailure",
			event: K8sEventData{
				Reason:  "Started",
				Message: "Container started successfully",
				Type:    "Normal",
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := detectDNSFailure(tt.event)

			if tt.shouldDetect {
				if pattern == nil {
					t.Fatal("Expected to detect DNS failure, got nil")
				}
				if pattern.PatternType != "DNSFailure" {
					t.Errorf("Expected PatternType DNSFailure, got %s", pattern.PatternType)
				}
				if pattern.ImpactScore != 0.30 {
					t.Errorf("Expected impact score 0.30, got %f", pattern.ImpactScore)
				}
				if tt.expectedHost != "" && !strings.Contains(pattern.Details, tt.expectedHost) {
					t.Errorf("Expected details to contain host '%s', got '%s'", tt.expectedHost, pattern.Details)
				}
			} else if pattern != nil {
				t.Errorf("Expected no detection, got pattern: %+v", pattern)
			}
		})
	}
}

func TestAnalyzeEventPatterns(t *testing.T) {
	events := []K8sEventData{
		{
			Reason:  "FailedScheduling",
			Message: "0/3 nodes available: 3 Insufficient cpu",
			Type:    "Warning",
		},
		{
			Reason:  "Evicted",
			Message: "The node was low on resource: memory",
			Type:    "Warning",
		},
		{
			Reason:  "Started",
			Message: "Container started",
			Type:    "Normal",
		},
		{
			Reason:  "Preempted",
			Message: "Pod was preempted",
			Type:    "Normal",
		},
	}

	patterns := AnalyzeEventPatterns(events)

	// Should detect 3 patterns (FailedScheduling, Evicted, Preempted)
	// "Started" should not create a pattern
	if len(patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(patterns))
	}

	// Check pattern types
	patternTypes := make(map[string]bool)
	for _, p := range patterns {
		patternTypes[p.PatternType] = true
	}

	expectedTypes := []string{"FailedScheduling", "Evicted", "Preempted"}
	for _, expected := range expectedTypes {
		if !patternTypes[expected] {
			t.Errorf("Expected to find pattern type %s", expected)
		}
	}
}

func TestGetHighestEventPatternImpact(t *testing.T) {
	patterns := []EventPattern{
		{PatternType: "FailedScheduling", ImpactScore: 0.30},
		{PatternType: "Evicted", ImpactScore: 0.35},
		{PatternType: "Preempted", ImpactScore: 0.25},
	}

	maxScore := GetHighestEventPatternImpact(patterns)
	if maxScore != 0.35 {
		t.Errorf("Expected highest score 0.35, got %f", maxScore)
	}
}

func TestGetHighestEventPatternImpact_NoPatterns(t *testing.T) {
	patterns := []EventPattern{}
	maxScore := GetHighestEventPatternImpact(patterns)
	if maxScore != 0.0 {
		t.Errorf("Expected 0.0 for no patterns, got %f", maxScore)
	}
}

func TestHasCriticalEventPattern(t *testing.T) {
	tests := []struct {
		name     string
		patterns []EventPattern
		expected bool
	}{
		{
			name: "FailedScheduling is critical",
			patterns: []EventPattern{
				{PatternType: "FailedScheduling"},
			},
			expected: true,
		},
		{
			name: "Evicted is critical",
			patterns: []EventPattern{
				{PatternType: "Evicted"},
			},
			expected: true,
		},
		{
			name: "Preempted is not critical",
			patterns: []EventPattern{
				{PatternType: "Preempted"},
			},
			expected: false,
		},
		{
			name: "DNSFailure is not critical",
			patterns: []EventPattern{
				{PatternType: "DNSFailure"},
			},
			expected: false,
		},
		{
			name:     "No patterns",
			patterns: []EventPattern{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasCriticalEventPattern(tt.patterns)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseSchedulingConstraints(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		containsText string // Text that should be present
	}{
		{
			name:         "Multiple constraints",
			message:      "0/5 nodes are available: 3 Insufficient cpu, 2 node selector mismatch",
			containsText: "node selector",
		},
		{
			name:         "Taint constraint",
			message:      "0/3 nodes are available: 1 node(s) had taint {key: value}, that the pod didn't tolerate",
			containsText: "taint",
		},
		{
			name:         "Single constraint",
			message:      "0/2 nodes are available: 2 Insufficient memory.",
			containsText: "Insufficient memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSchedulingConstraints(tt.message)
			if !strings.Contains(strings.ToLower(result), strings.ToLower(tt.containsText)) {
				t.Errorf("Expected result to contain '%s', got '%s'", tt.containsText, result)
			}
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "No such host",
			message:  "dial tcp: lookup api.example.com: no such host",
			expected: "api.example.com",
		},
		{
			name:     "Dial tcp lookup",
			message:  "dial tcp: lookup database.local failed",
			expected: "database.local",
		},
		{
			name:     "No hostname",
			message:  "Connection failed",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHostname(tt.message)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
