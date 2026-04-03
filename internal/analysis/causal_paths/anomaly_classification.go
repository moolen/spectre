package causalpaths

import (
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// causeIntroducingAnomalyTypes are anomaly types that can introduce failures.
// These represent changes or conditions that cause downstream problems.
var causeIntroducingAnomalyTypes = map[string]bool{
	// Change anomalies - configuration mutations.
	"ConfigChange":          true, // ConfigMap data modified
	"SecretChange":          true, // Secret data modified
	"ConfigMapModified":     true, // Legacy name (deprecated)
	"SecretModified":        true, // Legacy name (deprecated)
	"HelmReleaseUpdated":    true,
	"KustomizationUpdated":  true,
	"WorkloadSpecModified":  true,
	"ImageChanged":          true,
	"EnvironmentChanged":    true,
	"ResourceLimitsChanged": true,
	"RoleModified":          true,
	"RoleBindingModified":   true,
	"SpecModified":          true,
	"ResourceDeleted":       true,

	// State anomalies - resource-level issues that cause downstream failures.
	"NodeNotReady":        true,
	"DiskPressure":        true,
	"NodeDiskPressure":    true, // Legacy name (deprecated)
	"NodeMemoryPressure":  true,
	"NodePIDPressure":     true,
	"NodeNetworkPressure": true,
	"Evicted":             true, // Pod eviction can cause cascading failures (Job recreation, etc.)

	// Configuration anomalies - missing or invalid configuration that causes failures.
	"SecretMissing":          true, // Missing Secret/ConfigMap reference
	"InvalidConfigReference": true, // Invalid config reference (FailedMount due to missing resource)
	"CertExpired":            true, // Certificate has expired

	// RBAC anomalies - permission issues that cause downstream failures.
	"ServiceAccountMissing":      true, // Missing ServiceAccount reference
	"RBACDenied":                 true, // Forbidden event due to RBAC
	"ClusterRoleModified":        true, // ClusterRole spec changes
	"ClusterRoleBindingModified": true, // ClusterRoleBinding spec changes

	// Helm/GitOps anomalies - deployment pipeline issues that cause downstream failures.
	"HelmUpgrade":         true, // HelmRelease version upgraded
	"HelmRollback":        true, // HelmRelease version rolled back
	"ValuesChanged":       true, // HelmRelease values configuration changed
	"HelmReleaseFailed":   true, // HelmRelease Ready=False or Released=False
	"KustomizationFailed": true, // Kustomization Ready=False

	// Image and registry anomalies - image issues that cause Pod failures.
	"ImageNotFound":      true, // Image doesn't exist in registry
	"RegistryAuthFailed": true, // Registry authentication failed
	"ImagePullTimeout":   true, // Image pull timeout/connection error

	// Storage anomalies - volume/disk issues that cause Pod failures.
	"PVCBindingFailed":   true, // PVC failed to bind to PV
	"VolumeMountFailed":  true, // Volume mount failed (non-config)
	"VolumeOutOfSpace":   true, // Disk/volume space exhaustion
	"ReadOnlyFilesystem": true, // Filesystem mounted/became read-only
}

// derivedFailureAnomalyTypes are anomaly types that are symptoms, not causes.
// These represent the effects of upstream issues.
var derivedFailureAnomalyTypes = map[string]bool{
	"CrashLoopBackOff": true, // Result of upstream issue
	// Note: ImagePullBackOff is context-dependent - see ClassifyImagePullAnomaly().
	// It's intentionally omitted because it needs context-aware classification.
	"OOMKilled":           true, // Result of resource limits
	"PodFailed":           true, // Result of upstream issue
	"PodPending":          true, // Result of scheduling/resources
	"ErrorStatus":         true, // Generic error state
	"TerminatingStatus":   true, // Deletion in progress
	"CreateContainerErr":  true, // Container creation failed
	"InitContainerFailed": true, // Init container failure (usually symptom of config issue)

	// Deployment/ReplicaSet derived failures - never stop traversal here.
	"RolloutStuck":           true, // Deployment rollout stuck/ProgressDeadlineExceeded
	"ReplicaCreationFailure": true, // FailedCreate event on Deployment/ReplicaSet
}

// intermediateAnomalyTypes are cause-introducing anomalies that often represent
// intermediate effects rather than root causes. The causal path algorithm should
// continue past these to find deeper root causes.
var intermediateAnomalyTypes = map[string]bool{
	"ResourceCreated": true, // Often an intermediate effect of upstream changes (e.g., HelmRelease -> Deployment).
}

// IsCauseIntroducingAnomaly checks if an anomaly type can introduce failures.
func IsCauseIntroducingAnomaly(anomalyType string, category anomaly.AnomalyCategory) bool {
	if category == anomaly.CategoryChange {
		return true
	}

	return causeIntroducingAnomalyTypes[anomalyType]
}

// IsDerivedFailureAnomaly checks if an anomaly type is a symptom/effect.
func IsDerivedFailureAnomaly(anomalyType string) bool {
	return derivedFailureAnomalyTypes[anomalyType]
}

// HasCauseIntroducingAnomaly checks if a node has anomalies that could introduce failures.
// Anomaly must occur before or at the symptom's first failure time.
func HasCauseIntroducingAnomaly(nodeAnomalies []anomaly.Anomaly, symptomFirstFailure time.Time) bool {
	for _, a := range nodeAnomalies {
		if a.Timestamp.After(symptomFirstFailure) {
			continue
		}

		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return true
		}
	}

	return false
}

// hasOnlyIntermediateCauseAnomalies checks if a node has only intermediate cause-introducing
// anomalies (like ResourceCreated). These nodes should be traversed past to find deeper root causes.
func hasOnlyIntermediateCauseAnomalies(nodeAnomalies []anomaly.Anomaly) bool {
	if len(nodeAnomalies) == 0 {
		return false
	}

	hasIntermediateCause := false
	for _, a := range nodeAnomalies {
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			if !intermediateAnomalyTypes[a.Type] {
				return false
			}
			hasIntermediateCause = true
		}
	}

	return hasIntermediateCause
}

// HasOnlyDerivedAnomalies checks if a node has only derived failure anomalies.
// Returns true if there are no cause-introducing anomalies.
func HasOnlyDerivedAnomalies(nodeAnomalies []anomaly.Anomaly) bool {
	if len(nodeAnomalies) == 0 {
		return true
	}

	for _, a := range nodeAnomalies {
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return false
		}
	}

	return true
}

// GetFirstCauseIntroducingAnomaly returns the earliest cause-introducing anomaly.
func GetFirstCauseIntroducingAnomaly(nodeAnomalies []anomaly.Anomaly, beforeTime time.Time) *anomaly.Anomaly {
	var earliest *anomaly.Anomaly

	for i := range nodeAnomalies {
		a := &nodeAnomalies[i]

		if a.Timestamp.After(beforeTime) {
			continue
		}
		if !IsCauseIntroducingAnomaly(a.Type, a.Category) {
			continue
		}
		if earliest == nil || a.Timestamp.Before(earliest.Timestamp) {
			earliest = a
		}
	}

	return earliest
}
