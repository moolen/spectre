package anomaly

import (
	"strings"
)

// NetworkAnomalyDetector detects network-related anomalies
type NetworkAnomalyDetector struct{}

// NewNetworkAnomalyDetector creates a new network anomaly detector
func NewNetworkAnomalyDetector() *NetworkAnomalyDetector {
	return &NetworkAnomalyDetector{}
}

// Detect analyzes network conditions for anomalies
func (d *NetworkAnomalyDetector) Detect(input DetectorInput) []Anomaly {
	var anomalies []Anomaly

	// Check for network connectivity issues in events
	for _, event := range input.AllEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Check description for connection refused errors
		descLower := strings.ToLower(event.Description)
		if strings.Contains(descLower, "connection refused") ||
		   strings.Contains(descLower, "connection reset") ||
		   strings.Contains(descLower, "timeout") && strings.Contains(descLower, "connect") {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryNetwork,
				Type:      "ConnectionRefused",
				Severity:  SeverityHigh,
				Timestamp: event.Timestamp,
				Summary:   "Network connection refused or failed",
				Details: map[string]interface{}{
					"description": event.Description,
				},
			})
		}
	}

	// Check K8s events for network errors
	for _, event := range input.K8sEvents {
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		msgLower := strings.ToLower(event.Message)
		if strings.Contains(msgLower, "connection refused") ||
		   strings.Contains(msgLower, "network unreachable") ||
		   strings.Contains(msgLower, "no route to host") {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryNetwork,
				Type:      "ConnectionRefused",
				Severity:  SeverityHigh,
				Timestamp: event.Timestamp,
				Summary:   "Network connection issue detected",
				Details: map[string]interface{}{
					"event_message": event.Message,
					"event_reason":  event.Reason,
				},
			})
		}
	}

	return anomalies
}
