package anomaly

import (
	"fmt"
	"strings"

	"github.com/moolen/spectre/internal/analysis"
)

// EventAnomalyDetector detects anomalies from Kubernetes Event objects
type EventAnomalyDetector struct{}

// NewEventAnomalyDetector creates a new event anomaly detector
func NewEventAnomalyDetector() *EventAnomalyDetector {
	return &EventAnomalyDetector{}
}

// isBenignEventReason checks if an event reason is a normal operational event
func isBenignEventReason(reason string) bool {
	benignReasons := map[string]bool{
		"ScalingReplicaSet":     true, // Normal scaling operations
		"SuccessfulCreate":      true, // Normal pod/resource creation
		"SuccessfulDelete":      true, // Normal pod/resource deletion
		"Scheduled":             true, // Normal pod scheduling
		"Pulling":               true, // Normal image pulling
		"Pulled":                true, // Normal image pulled
		"Created":               true, // Normal container creation
		"Started":               true, // Normal container startup
		"NoPods":                true, // Informational, not an error
		"ProvisioningSucceeded": true, // Normal PVC provisioning
	}
	return benignReasons[reason]
}

// Detect analyzes K8s events for anomalies
func (d *EventAnomalyDetector) Detect(input DetectorInput) []Anomaly {
	var warningEvents []analysis.K8sEventInfo
	reasonCounts := make(map[string]int)
	reasonEvents := make(map[string][]int) // Track event counts for each reason

	for _, event := range input.K8sEvents {
		// Skip events outside time window
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Skip benign events (normal operations)
		if isBenignEventReason(event.Reason) {
			continue
		}

		// Count occurrences by reason
		reasonCounts[event.Reason]++
		reasonEvents[event.Reason] = append(reasonEvents[event.Reason], event.Count)

		// Collect Warning events for deduplication
		if event.Type == "Warning" {
			warningEvents = append(warningEvents, event)
		}

		// Rule: High event count within a single K8s event (excluding benign events)
		if event.Count > 10 {
			// Will be added to anomalies later
		}
	}

	// Process Warning events
	anomalies := make([]Anomaly, 0, len(warningEvents))
	seenEventReasons := make(map[string]bool) // Track to avoid duplicates with same reason

	for _, event := range warningEvents {
		// Skip if we've already processed this reason
		// (multiple K8s events with same reason are handled by RepeatedEvent logic)
		if seenEventReasons[event.Reason] {
			continue
		}
		seenEventReasons[event.Reason] = true

		// Create anomaly for this Warning event
		severity := ClassifyK8sEventSeverity(event.Reason)
		anomalyType := event.Reason

		// Map FailedCreate on Deployment/ReplicaSet to ReplicaCreationFailure
		// These are derived failures that indicate the controller couldn't create pods
		if event.Reason == "FailedCreate" {
			if input.Node.Resource.Kind == "Deployment" || input.Node.Resource.Kind == "ReplicaSet" {
				anomalyType = "ReplicaCreationFailure"
				severity = SeverityMedium // ReplicaCreationFailure is medium severity per taxonomy
			}
		}

		// Map FailedMount to InvalidConfigReference when it's a missing secret/configmap
		// This is a cause-introducing anomaly that indicates invalid configuration references
		if event.Reason == "FailedMount" {
			msg := strings.ToLower(event.Message)
			isSecretMissing := strings.Contains(msg, "secret") && (strings.Contains(msg, "not found") || strings.Contains(msg, "doesn't exist"))
			isConfigMapMissing := strings.Contains(msg, "configmap") && (strings.Contains(msg, "not found") || strings.Contains(msg, "doesn't exist"))
			if isSecretMissing || isConfigMapMissing {
				anomalyType = "InvalidConfigReference"
				severity = SeverityHigh // Per taxonomy
			}
		}

		// Map Forbidden events to RBACDenied
		// This is a cause-introducing anomaly indicating RBAC permission issues
		if event.Reason == "Forbidden" || event.Reason == "FailedCreate" {
			msg := strings.ToLower(event.Message)
			isForbidden := strings.Contains(msg, "forbidden") || strings.Contains(msg, "cannot") && strings.Contains(msg, "permission")
			if isForbidden {
				anomalyType = "RBACDenied"
				severity = SeverityHigh // Per taxonomy
			}
		}

		// Map image pull errors to specific anomaly types based on message content
		// These are cause-introducing anomalies that indicate image/registry issues
		if event.Reason == "Failed" || event.Reason == "ErrImagePull" || event.Reason == "ImagePullBackOff" {
			msg := strings.ToLower(event.Message)

			// ImageNotFound: image doesn't exist in registry
			isNotFound := strings.Contains(msg, "not found") ||
				strings.Contains(msg, "manifest unknown") ||
				strings.Contains(msg, "does not exist")
			if isNotFound {
				anomalyType = "ImageNotFound"
				severity = SeverityCritical
			}

			// RegistryAuthFailed: authentication/authorization errors
			isAuthFailed := strings.Contains(msg, "unauthorized") ||
				strings.Contains(msg, "forbidden") ||
				strings.Contains(msg, "authentication") ||
				strings.Contains(msg, "x509")
			if isAuthFailed {
				anomalyType = "RegistryAuthFailed"
				severity = SeverityCritical
			}

			// ImagePullTimeout: timeout/connection errors
			isTimeout := strings.Contains(msg, "i/o timeout") ||
				strings.Contains(msg, "context deadline exceeded") ||
				strings.Contains(msg, "connection refused")
			if isTimeout {
				anomalyType = "ImagePullTimeout"
				severity = SeverityHigh
			}
		}

		// Map storage/volume-related events to specific anomaly types
		// These are cause-introducing anomalies that indicate storage issues
		if event.Reason == "FailedMount" || event.Reason == "FailedAttachVolume" {
			msg := strings.ToLower(event.Message)

			// Check if it's NOT a missing config reference (already handled above)
			isSecretMissing := strings.Contains(msg, "secret") && (strings.Contains(msg, "not found") || strings.Contains(msg, "doesn't exist"))
			isConfigMapMissing := strings.Contains(msg, "configmap") && (strings.Contains(msg, "not found") || strings.Contains(msg, "doesn't exist"))

			if !isSecretMissing && !isConfigMapMissing {
				// VolumeMountFailed: general volume mount failures
				anomalyType = "VolumeMountFailed"
				severity = SeverityHigh
			}
		}

		// VolumeOutOfSpace: disk/volume space exhaustion
		if event.Reason == "Evicted" || event.Reason == "FreeDiskSpaceFailed" || event.Reason == "EvictionThresholdMet" {
			msg := strings.ToLower(event.Message)
			isSpaceIssue := strings.Contains(msg, "disk") ||
				strings.Contains(msg, "space") ||
				strings.Contains(msg, "ephemeral") ||
				strings.Contains(msg, "storage") ||
				strings.Contains(msg, "exceeded")
			if isSpaceIssue {
				anomalyType = "VolumeOutOfSpace"
				severity = SeverityHigh
			}
		}

		// Also check for space issues in Failed events
		if event.Reason == "Failed" || event.Reason == "Warning" {
			msg := strings.ToLower(event.Message)
			isSpaceIssue := (strings.Contains(msg, "no space left") ||
				strings.Contains(msg, "disk full") ||
				strings.Contains(msg, "out of disk") ||
				strings.Contains(msg, "exceeded ephemeral storage"))
			if isSpaceIssue {
				anomalyType = "VolumeOutOfSpace"
				severity = SeverityHigh
			}
		}

		// ReadOnlyFilesystem: filesystem mounted read-only or became read-only
		{
			msg := strings.ToLower(event.Message)
			isReadOnly := strings.Contains(msg, "read-only file system") ||
				strings.Contains(msg, "readonly file system") ||
				strings.Contains(msg, "read only file system") ||
				strings.Contains(msg, "erofs") ||
				(strings.Contains(msg, "remount") && strings.Contains(msg, "read-only"))
			if isReadOnly {
				anomalyType = "ReadOnlyFilesystem"
				severity = SeverityHigh
			}
		}

		anomaly := Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryEvent,
			Type:      anomalyType,
			Severity:  severity,
			Timestamp: event.Timestamp,
			Summary:   event.Message,
			Details: map[string]interface{}{
				"event_type": event.Type,
				"count":      event.Count,
				"source":     event.Source,
				"reason":     event.Reason,
			},
		}

		anomalies = append(anomalies, anomaly)
	}

	// Add high frequency events
	for _, event := range input.K8sEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}
		if isBenignEventReason(event.Reason) {
			continue
		}
		if event.Count > 10 {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryEvent,
				Type:      "HighFrequencyEvent",
				Severity:  SeverityHigh,
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("Event '%s' occurred %d times", event.Reason, event.Count),
				Details: map[string]interface{}{
					"reason": event.Reason,
					"count":  event.Count,
				},
			})
		}
	}

	// Rule: Repeated events (different event UIDs, same reason)
	for reason, count := range reasonCounts {
		if count >= 3 {
			// Calculate total count across all events with this reason
			totalCount := 0
			for _, c := range reasonEvents[reason] {
				totalCount += c
			}

			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryEvent,
				Type:      "RepeatedEvent",
				Severity:  SeverityHigh,
				Timestamp: input.TimeWindow.End,
				Summary:   fmt.Sprintf("Event reason '%s' occurred %d times (total count: %d)", reason, count, totalCount),
				Details: map[string]interface{}{
					"reason":      reason,
					"occurrences": count,
					"total_count": totalCount,
				},
			})
		}
	}

	return anomalies
}
