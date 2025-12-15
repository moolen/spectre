package storage

import (
	"encoding/json"
	"regexp"
	"strings"
)

// EventPattern represents a detected event pattern
type EventPattern struct {
	PatternType string  `json:"pattern_type"` // FailedScheduling, Evicted, Preempted, DNSFailure
	Reason      string  `json:"reason"`
	Message     string  `json:"message"`
	Details     string  `json:"details"` // Parsed details from the message
	ImpactScore float64 `json:"impact_score"`
}

// K8sEventData represents event data structure from Kubernetes
type K8sEventData struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Type    string `json:"type"` // Normal, Warning
}

// AnalyzeEventPatterns inspects Kubernetes events and detects specific patterns
func AnalyzeEventPatterns(events []K8sEventData) []EventPattern {
	patterns := make([]EventPattern, 0)

	for _, event := range events {
		// Check for scheduling failures
		if pattern := detectSchedulingFailure(event); pattern != nil {
			patterns = append(patterns, *pattern)
		}

		// Check for evictions
		if pattern := detectEviction(event); pattern != nil {
			patterns = append(patterns, *pattern)
		}

		// Check for preemptions
		if pattern := detectPreemption(event); pattern != nil {
			patterns = append(patterns, *pattern)
		}

		// Check for DNS failures
		if pattern := detectDNSFailure(event); pattern != nil {
			patterns = append(patterns, *pattern)
		}

		// Check for probe failures (Phase 3)
		if pattern := detectProbeFailure(event); pattern != nil {
			patterns = append(patterns, *pattern)
		}
	}

	return patterns
}

// detectSchedulingFailure detects FailedScheduling events and extracts constraints
// Examples:
// - "0/5 nodes available: 3 Insufficient cpu, 2 node selector mismatch"
// - "0/3 nodes are available: 1 node(s) had taint {key: value}, that the pod didn't tolerate"
func detectSchedulingFailure(event K8sEventData) *EventPattern {
	if !strings.Contains(event.Reason, "FailedScheduling") {
		return nil
	}

	details := parseSchedulingConstraints(event.Message)

	return &EventPattern{
		PatternType: "FailedScheduling",
		Reason:      event.Reason,
		Message:     event.Message,
		Details:     details,
		ImpactScore: 0.30,
	}
}

// parseSchedulingConstraints extracts the reasons why scheduling failed
func parseSchedulingConstraints(message string) string {
	// Common patterns in FailedScheduling messages
	patterns := []string{
		"Insufficient cpu",
		"Insufficient memory",
		"Insufficient pods",
		"node selector",
		"node affinity",
		"pod affinity",
		"taint",
		"toleration",
		"volume",
		"storage",
	}

	constraints := make([]string, 0)
	msgLower := strings.ToLower(message)

	for _, pattern := range patterns {
		if strings.Contains(msgLower, pattern) {
			constraints = append(constraints, pattern)
		}
	}

	if len(constraints) > 0 {
		return "Constraints: " + strings.Join(constraints, ", ")
	}

	// Extract the full reason from common format: "0/N nodes available: <reasons>"
	re := regexp.MustCompile(`0/\d+\s+nodes?\s+(?:are\s+)?available:\s*(.+)`)
	matches := re.FindStringSubmatch(message)
	if len(matches) > 1 {
		return matches[1]
	}

	return message
}

// detectEviction detects pod eviction events
// Reasons: Evicted
// Messages contain the cause (e.g., "The node was low on resource: memory")
func detectEviction(event K8sEventData) *EventPattern {
	if !strings.EqualFold(event.Reason, "Evicted") {
		return nil
	}

	cause := parseEvictionCause(event.Message)

	return &EventPattern{
		PatternType: "Evicted",
		Reason:      event.Reason,
		Message:     event.Message,
		Details:     cause,
		ImpactScore: 0.35,
	}
}

// parseEvictionCause extracts the reason for eviction from the message
func parseEvictionCause(message string) string {
	// Common eviction causes
	causes := map[string]string{
		"memory":    "Memory pressure",
		"disk":      "Disk pressure",
		"ephemeral": "Ephemeral storage pressure",
		"pid":       "PID pressure",
		"inodes":    "Inode pressure",
	}

	msgLower := strings.ToLower(message)
	for keyword, cause := range causes {
		if strings.Contains(msgLower, keyword) {
			return cause
		}
	}

	// Try to extract "low on resource: <resource>"
	re := regexp.MustCompile(`low on resource:\s*(\w+)`)
	matches := re.FindStringSubmatch(message)
	if len(matches) > 1 {
		return "Low on: " + matches[1]
	}

	return message
}

// detectPreemption detects pod preemption events
// Reason: Preempted
func detectPreemption(event K8sEventData) *EventPattern {
	if !strings.EqualFold(event.Reason, "Preempted") {
		return nil
	}

	return &EventPattern{
		PatternType: "Preempted",
		Reason:      event.Reason,
		Message:     event.Message,
		Details:     "Pod was preempted by scheduler",
		ImpactScore: 0.25,
	}
}

// detectDNSFailure detects DNS-related failures in events
// Looks for DNS errors in event messages
func detectDNSFailure(event K8sEventData) *EventPattern {
	msgLower := strings.ToLower(event.Message)

	// DNS failure indicators
	dnsKeywords := []string{
		"no such host",
		"dns",
		"lookup failed",
		"name resolution",
		"dial tcp: lookup",
		"i/o timeout",
	}

	hasDNSKeyword := false
	for _, keyword := range dnsKeywords {
		if strings.Contains(msgLower, keyword) {
			hasDNSKeyword = true
			break
		}
	}

	if !hasDNSKeyword {
		return nil
	}

	// Extract hostname if present
	hostname := extractHostname(event.Message)

	details := "DNS lookup failure"
	if hostname != "" {
		details = "DNS lookup failed for: " + hostname
	}

	return &EventPattern{
		PatternType: "DNSFailure",
		Reason:      event.Reason,
		Message:     event.Message,
		Details:     details,
		ImpactScore: 0.30,
	}
}

// extractHostname attempts to extract a hostname from an error message
func extractHostname(message string) string {
	// Try to match "lookup <hostname>: no such host"
	re := regexp.MustCompile(`lookup\s+([a-zA-Z0-9.-]+):\s*no such host`)
	matches := re.FindStringSubmatch(message)
	if len(matches) > 1 {
		return matches[1]
	}

	// Try to match "dial tcp: lookup <hostname>"
	re = regexp.MustCompile(`dial tcp:\s*lookup\s+([a-zA-Z0-9.-]+)`)
	matches = re.FindStringSubmatch(message)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// GetHighestEventPatternImpact returns the highest impact score from event patterns
func GetHighestEventPatternImpact(patterns []EventPattern) float64 {
	maxScore := 0.0
	for _, pattern := range patterns {
		if pattern.ImpactScore > maxScore {
			maxScore = pattern.ImpactScore
		}
	}
	return maxScore
}

// HasCriticalEventPattern returns true if any event pattern is critical
func HasCriticalEventPattern(patterns []EventPattern) bool {
	for _, pattern := range patterns {
		// FailedScheduling and Eviction are considered critical
		if pattern.PatternType == "FailedScheduling" || pattern.PatternType == "Evicted" {
			return true
		}
	}
	return false
}

// detectProbeFailure detects readiness, liveness, and startup probe failures
// Reasons: "Unhealthy"
// Messages contain "Readiness probe failed", "Liveness probe failed", "Startup probe failed"
func detectProbeFailure(event K8sEventData) *EventPattern {
	if !strings.EqualFold(event.Reason, "Unhealthy") && !strings.Contains(event.Message, "probe failed") {
		return nil
	}

	msgLower := strings.ToLower(event.Message)

	// Determine probe type and severity
	var probeType string
	var impactScore float64

	if strings.Contains(msgLower, "liveness probe failed") {
		probeType = "LivenessProbe"
		impactScore = 0.35 // Higher impact - will cause restarts
	} else if strings.Contains(msgLower, "readiness probe failed") {
		probeType = "ReadinessProbe"
		impactScore = 0.25 // Medium impact - affects traffic routing
	} else if strings.Contains(msgLower, "startup probe failed") {
		probeType = "StartupProbe"
		impactScore = 0.30 // Medium-high impact - prevents startup
	} else {
		// Generic probe failure
		probeType = "ProbeFailure"
		impactScore = 0.25
	}

	return &EventPattern{
		PatternType: probeType,
		Reason:      event.Reason,
		Message:     event.Message,
		Details:     "Probe health check failed",
		ImpactScore: impactScore,
	}
}

// AnalyzeEventsFromJSON is a convenience function to analyze events from JSON
func AnalyzeEventsFromJSON(eventsData json.RawMessage) ([]EventPattern, error) {
	var events []K8sEventData
	if err := json.Unmarshal(eventsData, &events); err != nil {
		return nil, err
	}
	return AnalyzeEventPatterns(events), nil
}
