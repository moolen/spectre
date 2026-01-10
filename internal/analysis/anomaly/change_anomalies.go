package anomaly

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moolen/spectre/internal/analysis"
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
				if kind == "ConfigMap" || kind == "Secret" {
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
			if kind == "ConfigMap" || kind == "Secret" {
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
			if kind == "HelmRelease" {
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

func (d *ChangeAnomalyDetector) classifyConfigChange(kind string) (string, Severity) {
	var anomalyType string

	switch kind {
	case "ConfigMap":
		anomalyType = "ConfigChange"
	case "Secret":
		anomalyType = "SecretChange"
	case "HelmRelease":
		// Default to HelmReleaseUpdated - specific types (HelmUpgrade, HelmRollback, ValuesChanged)
		// are detected separately in detectHelmReleaseChanges
		anomalyType = "HelmReleaseUpdated"
	case "Kustomization":
		anomalyType = "KustomizationUpdated"
	case "Role":
		anomalyType = "RoleModified"
	case "ClusterRole":
		anomalyType = "ClusterRoleModified"
	case "RoleBinding":
		anomalyType = "RoleBindingModified"
	case "ClusterRoleBinding":
		anomalyType = "ClusterRoleBindingModified"
	case "Deployment", "StatefulSet", "DaemonSet":
		anomalyType = "WorkloadSpecModified"
	default:
		anomalyType = "SpecModified"
	}

	// Use GetSeverity to allow kind-specific overrides
	severity := GetSeverity(CategoryChange, anomalyType, kind)
	return anomalyType, severity
}

func (d *ChangeAnomalyDetector) detectSpecificChanges(input DetectorInput, event analysis.ChangeEventInfo) []Anomaly {
	var anomalies []Anomaly

	kind := input.Node.Resource.Kind

	// Detect taint additions for Nodes
	if kind == "Node" {
		// Check diffs if available
		for _, diff := range event.Diff {
			// Check for taint additions: spec.taints array additions
			if strings.Contains(diff.Path, "spec.taints") && (diff.Op == "add" || diff.Op == "replace") {
				anomalies = append(anomalies, Anomaly{
					Node:      NodeFromGraphNode(input.Node),
					Category:  CategoryChange,
					Type:      "TaintAdded",
					Severity:  SeverityMedium,
					Timestamp: event.Timestamp,
					Summary:   "Node taint added",
					Details: map[string]interface{}{
						"path":      diff.Path,
						"new_value": diff.NewValue,
						"operation": diff.Op,
					},
				})
			}
		}

		// Alternative: detect from snapshot if no diffs available
		if len(event.Diff) == 0 && event.ConfigChanged {
			// Parse resource data to check for taints
			var resourceData map[string]interface{}
			if event.FullSnapshot != nil {
				resourceData = event.FullSnapshot
			} else if len(event.Data) > 0 {
				if err := json.Unmarshal(event.Data, &resourceData); err == nil {
					// Successfully parsed
				}
			}

			if resourceData != nil {
				if spec, ok := resourceData["spec"].(map[string]interface{}); ok {
					if taints, ok := spec["taints"].([]interface{}); ok && len(taints) > 0 {
						// Node has taints - if this is a config change event, assume taint was added
						anomalies = append(anomalies, Anomaly{
							Node:      NodeFromGraphNode(input.Node),
							Category:  CategoryChange,
							Type:      "TaintAdded",
							Severity:  SeverityMedium,
							Timestamp: event.Timestamp,
							Summary:   "Node taint added",
							Details: map[string]interface{}{
								"taint_count": len(taints),
							},
						})
					}
				}
			}
		}
	}

	for _, diff := range event.Diff {
		// Detect image changes
		// Check for either:
		// 1. Specific .image field changes (e.g., "spec.template.spec.containers[0].image")
		// 2. Entire containers array replacement (e.g., "spec.template.spec.containers")
		//    which commonly happens when images are updated in Kubernetes
		isImageFieldChange := strings.Contains(diff.Path, "spec.containers") && strings.HasSuffix(diff.Path, ".image")
		isContainersArrayChange := (diff.Path == "spec.containers" || diff.Path == "spec.template.spec.containers") && diff.Op == "replace"

		if isImageFieldChange || isContainersArrayChange {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "ImageChanged",
				Severity:  GetSeverity(CategoryChange, "ImageChanged", kind),
				Timestamp: event.Timestamp,
				Summary:   "Container image changed",
				Details: map[string]interface{}{
					"path":      diff.Path,
					"old_value": diff.OldValue,
					"new_value": diff.NewValue,
					"operation": diff.Op,
				},
			})
		}

		// Detect environment variable changes
		if strings.Contains(diff.Path, "spec.containers") && strings.Contains(diff.Path, ".env") {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "EnvironmentChanged",
				Severity:  GetSeverity(CategoryChange, "EnvironmentChanged", kind),
				Timestamp: event.Timestamp,
				Summary:   "Container environment variables changed",
				Details: map[string]interface{}{
					"path":      diff.Path,
					"operation": diff.Op,
				},
			})
		}

		// Detect resource limit/request changes
		if strings.Contains(diff.Path, "spec.containers") &&
			(strings.Contains(diff.Path, ".resources.limits") || strings.Contains(diff.Path, ".resources.requests")) {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "ResourceLimitsChanged",
				Severity:  GetSeverity(CategoryChange, "ResourceLimitsChanged", kind),
				Timestamp: event.Timestamp,
				Summary:   "Container resource limits/requests changed",
				Details: map[string]interface{}{
					"path":      diff.Path,
					"old_value": diff.OldValue,
					"new_value": diff.NewValue,
					"operation": diff.Op,
				},
			})
		}
	}

	return anomalies
}

func extractChangedFields(diffs []analysis.EventDiff) []string {
	fields := make([]string, 0, len(diffs))
	seen := make(map[string]bool)

	for _, diff := range diffs {
		// Deduplicate field paths
		if !seen[diff.Path] {
			fields = append(fields, diff.Path)
			seen[diff.Path] = true
		}
	}

	return fields
}

func extractReplicaChangeDetails(diffs []analysis.EventDiff) map[string]interface{} {
	details := make(map[string]interface{})

	for _, diff := range diffs {
		if strings.Contains(diff.Path, "replicas") || strings.Contains(diff.Path, "Replicas") {
			details["path"] = diff.Path
			details["old_value"] = diff.OldValue
			details["new_value"] = diff.NewValue
			details["operation"] = diff.Op
		}
	}

	return details
}

// isOnlyReplicaChange checks if the only changes are to replica fields
func isOnlyReplicaChange(changedFields []string) bool {
	if len(changedFields) == 0 {
		return false
	}

	for _, field := range changedFields {
		lowerField := strings.ToLower(field)
		// Check if field is NOT a replica field
		if !strings.Contains(lowerField, "replicas") {
			return false // Found a non-replica field, so not replica-only
		}
	}

	return true // All fields are replica-related
}

// areAllStatusChanges checks if all changes are to status fields (routine updates)
func areAllStatusChanges(changedFields []string) bool {
	if len(changedFields) == 0 {
		return false
	}

	for _, field := range changedFields {
		// Check if field is NOT a status field
		if !strings.HasPrefix(field, "status.") && !strings.HasPrefix(field, "status/") {
			return false // Found a non-status field
		}
	}

	return true // All fields are status fields
}

// isReplicaSetRoutineChange checks if changes are only routine ReplicaSet updates
// ReplicaSets managed by Deployments frequently have routine changes to:
// - metadata.annotations.deployment.kubernetes.io/* (revision tracking)
// - spec.replicas (scaling operations)
// - status.* (reconciliation state)
func isReplicaSetRoutineChange(kind string, changedFields []string) bool {
	// Only applies to ReplicaSets
	if kind != "ReplicaSet" {
		return false
	}

	if len(changedFields) == 0 {
		return false
	}

	for _, field := range changedFields {
		// Allow deployment-related metadata annotations
		if strings.HasPrefix(field, "metadata.annotations.deployment.kubernetes.io/") {
			continue
		}

		// Allow spec.replicas changes
		if field == "spec.replicas" {
			continue
		}

		// Allow status changes
		if strings.HasPrefix(field, "status.") || strings.HasPrefix(field, "status/") {
			continue
		}

		// Found a field that's NOT routine - this is a meaningful change
		return false
	}

	// All fields are routine ReplicaSet changes
	return true
}

// isWorkloadKind returns true for Kubernetes workload resources that manage Pods
// These are resources where a CREATE event is significant because if they're
// created with a misconfiguration, their descendant Pods will fail
func isWorkloadKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

// isRBACKind returns true for Kubernetes RBAC resources
// These are resources where CREATE/DELETE events are significant for causal analysis
// because RBAC changes can cause permission-related failures in Pods
func isRBACKind(kind string) bool {
	switch kind {
	case "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding":
		return true
	default:
		return false
	}
}

// shouldGenerateResourceCreated returns true for resource kinds where CREATE events
// should generate ResourceCreated anomalies for causal path analysis
func shouldGenerateResourceCreated(kind string) bool {
	return isWorkloadKind(kind) || isRBACKind(kind)
}

// detectHelmReleaseChanges detects specific HelmRelease change types:
// - HelmUpgrade: chart version or lastAppliedRevision increased
// - HelmRollback: version/revision decreased (rollback)
// - ValuesChanged: spec.values or spec.valuesFrom changed
func (d *ChangeAnomalyDetector) detectHelmReleaseChanges(input DetectorInput, event analysis.ChangeEventInfo) []Anomaly {
	var anomalies []Anomaly

	// Analyze diffs to detect specific change types
	var hasValuesChange bool
	var hasVersionChange bool
	var oldVersion, newVersion string

	for _, diff := range event.Diff {
		// Detect values changes: spec.values or spec.valuesFrom
		if strings.Contains(diff.Path, "spec.values") || strings.Contains(diff.Path, "spec.valuesFrom") {
			hasValuesChange = true
		}

		// Detect chart version changes in spec.chart.spec.version
		if strings.Contains(diff.Path, "spec.chart.spec.version") {
			hasVersionChange = true
			if s, ok := diff.OldValue.(string); ok {
				oldVersion = s
			}
			if s, ok := diff.NewValue.(string); ok {
				newVersion = s
			}
		}

		// Detect revision changes in status.lastAppliedRevision or status.lastAttemptedRevision
		if strings.Contains(diff.Path, "status.lastAppliedRevision") || strings.Contains(diff.Path, "status.lastAttemptedRevision") {
			// If we don't have a version from spec, use revision info
			if !hasVersionChange {
				hasVersionChange = true
				if s, ok := diff.OldValue.(string); ok {
					oldVersion = s
				}
				if s, ok := diff.NewValue.(string); ok {
					newVersion = s
				}
			}
		}
	}

	// Generate ValuesChanged anomaly
	if hasValuesChange {
		anomalies = append(anomalies, Anomaly{
			Node:      NodeFromGraphNode(input.Node),
			Category:  CategoryChange,
			Type:      "ValuesChanged",
			Severity:  GetSeverity(CategoryChange, "ValuesChanged", "HelmRelease"),
			Timestamp: event.Timestamp,
			Summary:   "HelmRelease values configuration changed",
			Details: map[string]interface{}{
				"event_type": event.EventType,
			},
		})
	}

	// Generate HelmUpgrade or HelmRollback anomaly based on version comparison
	if hasVersionChange && oldVersion != "" && newVersion != "" {
		isRollback := compareVersions(oldVersion, newVersion) > 0 // oldVersion > newVersion means rollback

		if isRollback {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "HelmRollback",
				Severity:  GetSeverity(CategoryChange, "HelmRollback", "HelmRelease"),
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("HelmRelease rolled back from %s to %s", oldVersion, newVersion),
				Details: map[string]interface{}{
					"old_version": oldVersion,
					"new_version": newVersion,
					"event_type":  event.EventType,
				},
			})
		} else {
			anomalies = append(anomalies, Anomaly{
				Node:      NodeFromGraphNode(input.Node),
				Category:  CategoryChange,
				Type:      "HelmUpgrade",
				Severity:  GetSeverity(CategoryChange, "HelmUpgrade", "HelmRelease"),
				Timestamp: event.Timestamp,
				Summary:   fmt.Sprintf("HelmRelease upgraded from %s to %s", oldVersion, newVersion),
				Details: map[string]interface{}{
					"old_version": oldVersion,
					"new_version": newVersion,
					"event_type":  event.EventType,
				},
			})
		}
	}

	return anomalies
}

// compareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
// Handles semver-like versions (e.g., "1.2.3") and revision numbers
func compareVersions(v1, v2 string) int {
	// Split by common version separators
	parts1 := strings.FieldsFunc(v1, func(r rune) bool { return r == '.' || r == '-' })
	parts2 := strings.FieldsFunc(v2, func(r rune) bool { return r == '.' || r == '-' })

	// Compare each part numerically when possible, otherwise lexicographically
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		// Try numeric comparison first
		n1, err1 := parseVersionNumber(p1)
		n2, err2 := parseVersionNumber(p2)

		if err1 == nil && err2 == nil {
			// Both are numeric
			if n1 < n2 {
				return -1
			} else if n1 > n2 {
				return 1
			}
		} else {
			// Fall back to lexicographic comparison
			if p1 < p2 {
				return -1
			} else if p1 > p2 {
				return 1
			}
		}
	}

	return 0
}

// parseVersionNumber extracts a numeric value from a version part
// Handles cases like "v1" -> 1, "1" -> 1
func parseVersionNumber(s string) (int, error) {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	var num int
	_, err := fmt.Sscanf(s, "%d", &num)
	return num, err
}
