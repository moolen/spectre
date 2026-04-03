package anomaly

import (
	"fmt"
	"strings"

	"github.com/moolen/spectre/internal/analysis"
)

func (d *ChangeAnomalyDetector) classifyConfigChange(kind string) (string, Severity) {
	var anomalyType string

	switch kind {
	case kindConfigMap:
		anomalyType = "ConfigChange"
	case "Secret":
		anomalyType = "SecretChange"
	case kindHelmRelease:
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
	case kindDeployment, "StatefulSet", "DaemonSet":
		anomalyType = "WorkloadSpecModified"
	default:
		anomalyType = "SpecModified"
	}

	return anomalyType, GetSeverity(CategoryChange, anomalyType, kind)
}

func extractChangedFields(diffs []analysis.EventDiff) []string {
	fields := make([]string, 0, len(diffs))
	seen := make(map[string]bool)

	for _, diff := range diffs {
		if seen[diff.Path] {
			continue
		}
		fields = append(fields, diff.Path)
		seen[diff.Path] = true
	}

	return fields
}

// isOnlyReplicaChange checks if the only changes are to replica fields
func isOnlyReplicaChange(changedFields []string) bool {
	if len(changedFields) == 0 {
		return false
	}

	for _, field := range changedFields {
		if !strings.Contains(strings.ToLower(field), "replicas") {
			return false
		}
	}

	return true
}

// areAllStatusChanges checks if all changes are to status fields (routine updates)
func areAllStatusChanges(changedFields []string) bool {
	if len(changedFields) == 0 {
		return false
	}

	for _, field := range changedFields {
		if !strings.HasPrefix(field, "status.") && !strings.HasPrefix(field, "status/") {
			return false
		}
	}

	return true
}

// isReplicaSetRoutineChange checks if changes are only routine ReplicaSet updates.
func isReplicaSetRoutineChange(kind string, changedFields []string) bool {
	if kind != kindReplicaSet || len(changedFields) == 0 {
		return false
	}

	for _, field := range changedFields {
		if strings.HasPrefix(field, "metadata.annotations.deployment.kubernetes.io/") {
			continue
		}
		if field == "spec.replicas" {
			continue
		}
		if strings.HasPrefix(field, "status.") || strings.HasPrefix(field, "status/") {
			continue
		}
		return false
	}

	return true
}

// isWorkloadKind returns true for Kubernetes workload resources that manage Pods.
func isWorkloadKind(kind string) bool {
	switch kind {
	case kindDeployment, "StatefulSet", "DaemonSet", kindReplicaSet, "Job", "CronJob":
		return true
	default:
		return false
	}
}

// isRBACKind returns true for Kubernetes RBAC resources.
func isRBACKind(kind string) bool {
	switch kind {
	case "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding":
		return true
	default:
		return false
	}
}

// shouldGenerateResourceCreated returns true for kinds where CREATE events
// should generate ResourceCreated anomalies for causal path analysis.
func shouldGenerateResourceCreated(kind string) bool {
	return isWorkloadKind(kind) || isRBACKind(kind)
}

// compareVersions compares two version strings.
func compareVersions(v1, v2 string) int {
	parts1 := strings.FieldsFunc(v1, func(r rune) bool { return r == '.' || r == '-' })
	parts2 := strings.FieldsFunc(v2, func(r rune) bool { return r == '.' || r == '-' })

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

		n1, err1 := parseVersionNumber(p1)
		n2, err2 := parseVersionNumber(p2)
		if err1 == nil && err2 == nil {
			if n1 < n2 {
				return -1
			}
			if n1 > n2 {
				return 1
			}
			continue
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// parseVersionNumber extracts a numeric value from a version part.
func parseVersionNumber(s string) (int, error) {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	var num int
	_, err := fmt.Sscanf(s, "%d", &num)
	return num, err
}
