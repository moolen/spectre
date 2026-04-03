package anomaly

import (
	"fmt"
)

const (
	kindConfigMap   = "ConfigMap"
	kindDeployment  = "Deployment"
	kindHelmRelease = "HelmRelease"
	kindReplicaSet  = "ReplicaSet"
)

// ChangeAnomalyDetector detects resource mutations and changes
type ChangeAnomalyDetector struct{}

// NewChangeAnomalyDetector creates a new change anomaly detector
func NewChangeAnomalyDetector() *ChangeAnomalyDetector {
	return &ChangeAnomalyDetector{}
}

// Detect analyzes resource changes for anomalies
func (d *ChangeAnomalyDetector) Detect(input DetectorInput) []Anomaly {
	var anomalies []Anomaly
	kind := input.Node.Resource.Kind

	for _, event := range input.AllEvents {
		// Skip events outside time window
		if event.Timestamp.Before(input.TimeWindow.Start) || event.Timestamp.After(input.TimeWindow.End) {
			continue
		}

		// Detect spec changes
		if event.ConfigChanged {
			// Extract changed fields from diff
			changedFields := extractChangedFields(event.Diff)

			// Skip if we don't have diff information
			// This happens when querying from the database which only stores full snapshots
			if len(event.Diff) == 0 && len(changedFields) == 0 {
				// We know config changed but don't have the specific fields
				// Still report it but without changed_fields detail
				anomType, severity := d.classifyConfigChange(kind)

				// Generate appropriate summary based on resource type
				var summary string
				if kind == "ConfigMap" || kind == kindSecret {
					summary = fmt.Sprintf("%s data modified", kind)
				} else {
					summary = fmt.Sprintf("%s configuration modified", kind)
				}

				anomalies = append(anomalies, Anomaly{
					Node:      NodeFromGraphNode(input.Node),
					Category:  CategoryChange,
					Type:      anomType,
					Severity:  severity,
					Timestamp: event.Timestamp,
					Summary:   summary,
					Details: map[string]interface{}{
						"event_type": event.EventType,
						// Note: changed_fields not available (diff not stored in database)
					},
				})
				continue
			}

			// Skip if ONLY replicas changed (normal scaling operations)
			if isOnlyReplicaChange(changedFields) {
				continue
			}

			// Skip if all changes are status fields (normal status updates)
			if areAllStatusChanges(changedFields) {
				continue
			}

			// Skip if ReplicaSet has only routine changes (metadata annotations, replicas, status)
			if isReplicaSetRoutineChange(kind, changedFields) {
				continue
			}

			anomType, severity := d.classifyConfigChange(kind)

			// Generate appropriate summary based on resource type
			var summary string
			if kind == kindConfigMap || kind == kindSecret {
				summary = fmt.Sprintf("%s data modified", kind)
			} else {
				summary = fmt.Sprintf("%s configuration modified", kind)
			}

			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      anomType,
				Severity:  severity,
				Timestamp: event.Timestamp,
				Summary:   summary,
				Details: map[string]interface{}{
					"changed_fields": changedFields,
					"event_type":     event.EventType,
				},
			})

			// Check for specific high-impact changes
			anomalies = append(anomalies, d.detectSpecificChanges(input, event)...)

			// Check for HelmRelease-specific changes (upgrade, rollback, values changed)
			if kind == kindHelmRelease {
				anomalies = append(anomalies, d.detectHelmReleaseChanges(input, event)...)
			}
		}

		// Detect deletes
		if event.EventType == "DELETE" {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "ResourceDeleted",
				Severity:  GetSeverity(CategoryChange, "ResourceDeleted", kind),
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("%s was deleted", kind),
				Details:   map[string]interface{}{},
			})
		}

		// Detect resource creation (helps establish causal paths for resources created broken)
		// When a workload or RBAC resource is created and causes downstream failures, the CREATE event is the root cause
		if event.EventType == "CREATE" && shouldGenerateResourceCreated(kind) {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "ResourceCreated",
				Severity:  GetSeverity(CategoryChange, "ResourceCreated", kind),
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("%s was created", kind),
				Details:   map[string]interface{}{},
			})
		}
	}

	return anomalies
}
