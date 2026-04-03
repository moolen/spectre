package causalpaths

import "github.com/moolen/spectre/internal/analysis/anomaly"

// reconciliationEffectOnManagedWorkloads lists anomaly types that are typically
// effects of GitOps reconciliation rather than root causes.
var reconciliationEffectOnManagedWorkloads = map[string]bool{
	"ImageChanged":          true, // Image change came from manager
	"SpecModified":          true, // Spec change came from manager
	"ResourceCreated":       true, // Resource was created by manager
	"EnvironmentChanged":    true, // Env vars changed by manager
	"ResourceLimitsChanged": true, // Limits changed by manager
	"WorkloadSpecModified":  true, // Workload spec changed by manager
}

// managedWorkloadKinds are Kubernetes resource types that can be managed by GitOps controllers.
var managedWorkloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"Job":         true,
	"CronJob":     true,
	"ConfigMap":   true,
	"Secret":      true,
	"Service":     true,
	"Ingress":     true,
}

// IsReconciliationEffectAnomaly checks if anomalies on a resource are likely
// effects of GitOps reconciliation rather than root causes.
func IsReconciliationEffectAnomaly(anomalies []anomaly.Anomaly, resourceKind string, hasUpstreamManager bool) bool {
	if !managedWorkloadKinds[resourceKind] || !hasUpstreamManager {
		return false
	}

	for _, a := range anomalies {
		if !reconciliationEffectOnManagedWorkloads[a.Type] {
			return false
		}
	}

	return len(anomalies) > 0
}
