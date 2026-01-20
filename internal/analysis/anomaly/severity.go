package anomaly

// SeverityRule defines a deterministic severity assignment
type SeverityRule struct {
	Category AnomalyCategory
	Type     string
	Kind     string // Optional: if specified, rule only applies to this resource kind
	Severity Severity
}

// severityRules is the canonical severity mapping
// This is the single source of truth for severity classification
var severityRules = []SeverityRule{
	// Kind-specific overrides (checked first)
	{CategoryChange, "SpecModified", "ReplicaSet", SeverityLow}, // ReplicaSets are managed by Deployments

	// Critical - Actively breaking workloads
	{CategoryState, "CrashLoopBackOff", "", SeverityCritical},
	{CategoryState, "ImagePullBackOff", "", SeverityCritical},
	{CategoryState, "PodFailed", "", SeverityCritical},
	{CategoryState, "NodeNotReady", "", SeverityCritical},
	{CategoryState, "SecretMissing", "", SeverityCritical},         // Missing Secret/ConfigMap reference
	{CategoryState, "CertExpired", "", SeverityCritical},           // Certificate has expired
	{CategoryState, "ServiceAccountMissing", "", SeverityCritical}, // Missing ServiceAccount reference
	{CategoryState, "HelmReleaseFailed", "", SeverityCritical},     // HelmRelease Ready=False or Released=False
	{CategoryState, "KustomizationFailed", "", SeverityCritical},   // Kustomization Ready=False
	{CategoryState, "PVCBindingFailed", "", SeverityCritical},      // PVC failed to bind to PV
	{CategoryEvent, "FailedScheduling", "", SeverityCritical},
	{CategoryEvent, "FailedMount", "", SeverityCritical},
	{CategoryEvent, "FailedAttachVolume", "", SeverityCritical},
	{CategoryEvent, "FailedCreatePodSandBox", "", SeverityCritical},
	{CategoryEvent, "FailedCreatePodContainer", "", SeverityCritical},
	{CategoryEvent, "NetworkNotReady", "", SeverityCritical},
	{CategoryEvent, "ImageNotFound", "", SeverityCritical},      // Image doesn't exist in registry
	{CategoryEvent, "RegistryAuthFailed", "", SeverityCritical}, // Registry authentication failed

	// High - Likely contributors
	{CategoryState, "OOMKilled", "", SeverityHigh},
	{CategoryState, "Deleted", "", SeverityHigh},
	{CategoryState, "ErrorStatus", "", SeverityHigh},
	{CategoryState, "NodeDiskPressure", "", SeverityHigh},
	{CategoryState, "NodeMemoryPressure", "", SeverityHigh},
	{CategoryState, "NodePIDPressure", "", SeverityHigh},
	{CategoryState, "PodPending", "", SeverityHigh},
	{CategoryState, "Evicted", "", SeverityHigh},
	{CategoryState, "ImagePullError", "", SeverityHigh},
	{CategoryState, "ErrImagePull", "", SeverityHigh},
	{CategoryState, "ContainerCreateError", "", SeverityHigh},
	{CategoryState, "InitContainerFailed", "", SeverityHigh},
	{CategoryState, "RolloutStuck", "", SeverityHigh}, // Deployment rollout stuck/ProgressDeadlineExceeded
	{CategoryEvent, "BackOff", "", SeverityHigh},
	{CategoryEvent, "FailedCreate", "", SeverityHigh},
	{CategoryEvent, "RepeatedEvent", "", SeverityHigh},
	{CategoryEvent, "HighFrequencyEvent", "", SeverityHigh},
	{CategoryEvent, "Unhealthy", "", SeverityHigh},
	{CategoryEvent, "Evicted", "", SeverityHigh},
	{CategoryEvent, "InvalidConfigReference", "", SeverityHigh}, // FailedMount due to missing Secret/ConfigMap
	{CategoryEvent, "RBACDenied", "", SeverityHigh},             // Forbidden event due to RBAC
	{CategoryEvent, "Forbidden", "", SeverityHigh},              // Raw Forbidden event
	{CategoryEvent, "ImagePullTimeout", "", SeverityHigh},       // Image pull timeout/connection error
	{CategoryEvent, "VolumeMountFailed", "", SeverityHigh},      // Volume mount failed (non-config)
	{CategoryEvent, "VolumeOutOfSpace", "", SeverityHigh},       // Disk/volume space exhaustion
	{CategoryEvent, "ReadOnlyFilesystem", "", SeverityHigh},     // Filesystem mounted/became read-only
	{CategoryChange, "ConfigMapModified", "", SeverityHigh},
	{CategoryChange, "SecretModified", "", SeverityHigh},
	{CategoryChange, "HelmReleaseUpdated", "", SeverityHigh},
	{CategoryChange, "HelmUpgrade", "", SeverityHigh},    // HelmRelease version upgraded
	{CategoryChange, "HelmRollback", "", SeverityMedium}, // HelmRelease version rolled back
	{CategoryChange, "ValuesChanged", "", SeverityHigh},  // HelmRelease values configuration changed
	{CategoryChange, "KustomizationUpdated", "", SeverityHigh},
	{CategoryChange, "ImageChanged", "", SeverityHigh},
	{CategoryChange, "RoleModified", "", SeverityHigh},
	{CategoryChange, "ClusterRoleModified", "", SeverityHigh}, // ClusterRole spec changes
	{CategoryChange, "RoleBindingModified", "", SeverityHigh},
	{CategoryChange, "ClusterRoleBindingModified", "", SeverityHigh}, // ClusterRoleBinding spec changes
	{CategoryChange, "WorkloadSpecModified", "", SeverityHigh},
	{CategoryFrequency, "HighRestartCount", "", SeverityHigh},
	{CategoryFrequency, "FlappingState", "", SeverityHigh},

	// Medium - Potential contributors
	{CategoryState, "TerminatingStatus", "", SeverityMedium},
	{CategoryEvent, "WarningEvent", "", SeverityMedium},
	{CategoryEvent, "Killing", "", SeverityMedium},
	{CategoryEvent, "Preempting", "", SeverityMedium},
	{CategoryEvent, "ReplicaCreationFailure", "", SeverityMedium}, // FailedCreate on Deployment/ReplicaSet
	{CategoryChange, "ReplicasChanged", "", SeverityMedium},
	{CategoryChange, "ResourceDeleted", "", SeverityMedium},
	{CategoryChange, "SpecModified", "", SeverityMedium}, // Default for most resources
	{CategoryChange, "EnvironmentChanged", "", SeverityMedium},
	{CategoryChange, "ResourceLimitsChanged", "", SeverityMedium},
	{CategoryFrequency, "RapidEvents", "", SeverityMedium},
	{CategoryFrequency, "ReconcileLoop", "", SeverityMedium},

	// Low - Informational
	{CategoryChange, "ResourceCreated", "", SeverityLow},
	{CategoryEvent, "NormalEvent", "", SeverityLow},
}

// GetSeverity returns the deterministic severity for an anomaly type
// If kind is provided, kind-specific rules are checked first before generic rules
func GetSeverity(category AnomalyCategory, anomalyType, kind string) Severity {
	// First pass: check for kind-specific rules
	if kind != "" {
		for _, rule := range severityRules {
			if rule.Kind != "" && rule.Category == category && rule.Type == anomalyType && rule.Kind == kind {
				return rule.Severity
			}
		}
	}

	// Second pass: check for generic rules (Kind == "")
	for _, rule := range severityRules {
		if rule.Kind == "" && rule.Category == category && rule.Type == anomalyType {
			return rule.Severity
		}
	}

	// Default to medium for unknown types
	return SeverityMedium
}

// K8sEventSeverityMap maps Kubernetes event reasons to their severity levels
var K8sEventSeverityMap = map[string]Severity{
	// Critical - actively breaking workloads
	"FailedScheduling":         SeverityCritical,
	"FailedMount":              SeverityCritical,
	"FailedAttachVolume":       SeverityCritical,
	"FailedCreatePodSandBox":   SeverityCritical,
	"FailedCreatePodContainer": SeverityCritical,
	"NetworkNotReady":          SeverityCritical,

	// High - likely contributors
	"BackOff":                SeverityHigh,
	"CrashLoopBackOff":       SeverityHigh,
	"ImagePullBackOff":       SeverityHigh,
	"ErrImagePull":           SeverityHigh,
	"FailedCreate":           SeverityHigh,
	"OOMKilled":              SeverityHigh,
	"NodeNotReady":           SeverityHigh,
	"Unhealthy":              SeverityHigh,
	"FailedSync":             SeverityHigh,
	"FailedValidation":       SeverityHigh,
	"Evicted":                SeverityHigh,
	"InvalidConfigReference": SeverityHigh,     // FailedMount due to missing Secret/ConfigMap
	"RBACDenied":             SeverityHigh,     // Forbidden event due to RBAC
	"Forbidden":              SeverityHigh,     // Raw Forbidden event
	"ImageNotFound":          SeverityCritical, // Image doesn't exist in registry
	"RegistryAuthFailed":     SeverityCritical, // Registry authentication failed
	"ImagePullTimeout":       SeverityHigh,     // Image pull timeout/connection error
	"VolumeMountFailed":      SeverityHigh,     // Volume mount failed (non-config)
	"VolumeOutOfSpace":       SeverityHigh,     // Disk/volume space exhaustion
	"ReadOnlyFilesystem":     SeverityHigh,     // Filesystem mounted/became read-only

	// Medium - potential contributors
	"Killing":             SeverityMedium,
	"Preempting":          SeverityMedium,
	"FreeDiskSpaceFailed": SeverityMedium,
	"InsufficientMemory":  SeverityMedium,
	"InsufficientCPU":     SeverityMedium,

	// Low - informational
	"Pulled":            SeverityLow,
	"Scheduled":         SeverityLow,
	"Started":           SeverityLow,
	"Created":           SeverityLow,
	"ScalingReplicaSet": SeverityLow,
	"SuccessfulCreate":  SeverityLow,
	"SuccessfulDelete":  SeverityLow,
	"SuccessfulUpdate":  SeverityLow,
}

// ClassifyK8sEventSeverity returns the severity for a K8s event reason
func ClassifyK8sEventSeverity(reason string) Severity {
	if severity, ok := K8sEventSeverityMap[reason]; ok {
		return severity
	}
	// Default to medium for unknown event reasons
	return SeverityMedium
}
