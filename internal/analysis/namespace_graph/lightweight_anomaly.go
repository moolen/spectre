package namespacegraph

import (
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// LightweightAnomalyDetector detects anomalies from already-fetched node data
// without making additional database queries. This is much faster than the full
// anomaly detection which requires causal graph traversal for each resource.
type LightweightAnomalyDetector struct{}

// NewLightweightAnomalyDetector creates a new lightweight anomaly detector
func NewLightweightAnomalyDetector() *LightweightAnomalyDetector {
	return &LightweightAnomalyDetector{}
}

// DetectFromNodes analyzes nodes using their already-fetched latest events
// and returns anomalies without making any database queries
func (d *LightweightAnomalyDetector) DetectFromNodes(nodes []Node, timestamp int64) []anomaly.Anomaly {
	var anomalies []anomaly.Anomaly
	seen := make(map[string]bool)

	for _, node := range nodes {
		nodeAnomalies := d.detectNodeAnomalies(node, timestamp)
		for _, a := range nodeAnomalies {
			key := a.Node.UID + ":" + string(a.Category) + ":" + a.Type
			if !seen[key] {
				seen[key] = true
				anomalies = append(anomalies, a)
			}
		}
	}

	return anomalies
}

// detectNodeAnomalies detects anomalies for a single node based on its latest event
func (d *LightweightAnomalyDetector) detectNodeAnomalies(node Node, timestamp int64) []anomaly.Anomaly {
	var anomalies []anomaly.Anomaly

	// Skip if no latest event
	if node.LatestEvent == nil {
		return anomalies
	}

	event := node.LatestEvent
	ts := time.Unix(0, timestamp)

	// Create base anomaly node info
	anomalyNode := anomaly.AnomalyNode{
		UID:       node.UID,
		Kind:      node.Kind,
		Name:      node.Name,
		Namespace: node.Namespace,
	}

	// Detect anomalies based on status
	switch event.Status {
	case StatusError:
		anomalies = append(anomalies, anomaly.Anomaly{
			Node:      anomalyNode,
			Category:  anomaly.CategoryState,
			Type:      "ErrorStatus",
			Severity:  anomaly.SeverityHigh,
			Timestamp: ts,
			Summary:   "Resource is in error state",
			Details: map[string]interface{}{
				"errorMessage": event.ErrorMessage,
			},
		})

	case StatusWarning:
		anomalies = append(anomalies, anomaly.Anomaly{
			Node:      anomalyNode,
			Category:  anomaly.CategoryState,
			Type:      "WarningStatus",
			Severity:  anomaly.SeverityMedium,
			Timestamp: ts,
			Summary:   "Resource is in warning state",
			Details: map[string]interface{}{
				"errorMessage": event.ErrorMessage,
			},
		})

	case StatusTerminating:
		anomalies = append(anomalies, anomaly.Anomaly{
			Node:      anomalyNode,
			Category:  anomaly.CategoryState,
			Type:      "Terminating",
			Severity:  anomaly.SeverityLow,
			Timestamp: ts,
			Summary:   "Resource is being terminated",
			Details:   map[string]interface{}{},
		})
	}

	// Detect container-specific issues (for Pods)
	if node.Kind == "Pod" && len(event.ContainerIssues) > 0 {
		for _, issue := range event.ContainerIssues {
			severity := d.getIssueSeverity(issue)
			anomalies = append(anomalies, anomaly.Anomaly{
				Node:      anomalyNode,
				Category:  anomaly.CategoryState,
				Type:      issue,
				Severity:  severity,
				Timestamp: ts,
				Summary:   "Container issue detected: " + issue,
				Details: map[string]interface{}{
					"issue": issue,
				},
			})
		}
	}

	// Detect high impact score
	if event.ImpactScore >= 0.7 {
		anomalies = append(anomalies, anomaly.Anomaly{
			Node:      anomalyNode,
			Category:  anomaly.CategoryState,
			Type:      "HighImpact",
			Severity:  anomaly.SeverityHigh,
			Timestamp: ts,
			Summary:   "Resource has high impact score",
			Details: map[string]interface{}{
				"impactScore": event.ImpactScore,
			},
		})
	}

	return anomalies
}

// getIssueSeverity returns the severity for a container issue type
func (d *LightweightAnomalyDetector) getIssueSeverity(issue string) anomaly.Severity {
	criticalIssues := map[string]bool{
		"CrashLoopBackOff": true,
		"OOMKilled":        true,
		"Error":            true,
	}

	highIssues := map[string]bool{
		"ImagePullBackOff":           true,
		"ErrImagePull":               true,
		"CreateContainerConfigError": true,
		"InvalidImageName":           true,
	}

	if criticalIssues[issue] {
		return anomaly.SeverityCritical
	}
	if highIssues[issue] {
		return anomaly.SeverityHigh
	}
	return anomaly.SeverityMedium
}
