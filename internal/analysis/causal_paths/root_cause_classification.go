package causalpaths

import "github.com/moolen/spectre/internal/analysis/anomaly"

// intentOwnerKinds are resource types that directly encode deployment or platform intent.
var intentOwnerKinds = map[string]bool{
	// GitOps controllers - highest priority intent owners.
	"HelmRelease":   true,
	"Kustomization": true,
	"Application":   true,

	// Infrastructure resources.
	"Node": true,

	// Configuration resources.
	"ConfigMap": true,
	"Secret":    true,

	// Storage resources.
	"PersistentVolume":      true,
	"PersistentVolumeClaim": true,

	// RBAC resources.
	"ClusterRole":        true,
	"Role":               true,
	"ClusterRoleBinding": true,
	"RoleBinding":        true,
	"ServiceAccount":     true,
}

// gitOpsControllerKinds are the subset of intent owners that are GitOps controllers.
var gitOpsControllerKinds = map[string]bool{
	"HelmRelease":   true,
	"Kustomization": true,
	"Application":   true,
}

// definitiveRootCauseAnomalyTypes mark resources as likely root causes rather than intermediates.
var definitiveRootCauseAnomalyTypes = map[string]bool{
	// Deletion anomalies - resource was removed.
	"ResourceDeleted":       true,
	"Deleted":               true,
	"TerminatingStatus":     true,
	"SecretMissing":         true,
	"ConfigMapMissing":      true,
	"ServiceAccountMissing": true,

	// Node pressure anomalies - infrastructure-level root causes.
	"NodeNotReady":        true,
	"DiskPressure":        true,
	"NodeDiskPressure":    true,
	"NodeMemoryPressure":  true,
	"NodePIDPressure":     true,
	"NodeNetworkPressure": true,

	// Configuration anomalies that are definitive causes.
	"CertExpired":            true,
	"InvalidConfigReference": true,
}

// IsIntentOwner returns true if the given resource kind is an intent owner.
func IsIntentOwner(kind string) bool {
	return intentOwnerKinds[kind]
}

// IsGitOpsController returns true if the given resource kind is a GitOps controller.
func IsGitOpsController(kind string) bool {
	return gitOpsControllerKinds[kind]
}

// HasDefinitiveRootCauseAnomaly checks if a node has anomalies that indicate it is
// a definitive root cause rather than an intermediate effect.
func HasDefinitiveRootCauseAnomaly(anomalies []anomaly.Anomaly) bool {
	for _, a := range anomalies {
		if definitiveRootCauseAnomalyTypes[a.Type] {
			return true
		}
	}

	return false
}
