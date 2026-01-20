package causalpaths

import (
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
)

const (
	reasonImagePullBackOff    = "ImagePullBackOff"
	anomalyTypeErrImagePull   = "ErrImagePull"
)

// edgeClassification maps relationship types to their causal category
// Cause-introducing edges represent relationships where changes propagate causally
// Materialization edges represent structural/ownership relationships
var edgeClassification = map[string]string{
	// Cause-Introducing Edges (changes propagate causally)
	"MANAGES":         EdgeCategoryCauseIntroducing, // HelmRelease/Kustomization manages lifecycle (special direction handling in buildUpstreamAdjacency)
	"TRIGGERED_BY":    EdgeCategoryCauseIntroducing, // Explicit causal trigger
	"REFERENCES_SPEC": EdgeCategoryCauseIntroducing, // ConfigMap/Secret reference in spec

	// RBAC Edges - Cause-Introducing (RBAC permission changes propagate to Pods)
	// Direction: RoleBinding --GRANTS_TO--> ServiceAccount, RoleBinding --BINDS_ROLE--> Role
	// Causal flow: Role change -> affects RoleBinding -> affects ServiceAccount permissions -> Pod fails
	"USES_SERVICE_ACCOUNT": EdgeCategoryCauseIntroducing, // Pod uses ServiceAccount (SA permissions affect Pod)
	"GRANTS_TO":            EdgeCategoryCauseIntroducing, // RoleBinding grants to ServiceAccount (special direction handling in buildUpstreamAdjacency)
	"BINDS_ROLE":           EdgeCategoryCauseIntroducing, // RoleBinding binds Role

	// Materialization Edges (structural/scheduling relationships)
	"OWNS":             EdgeCategoryMaterialization, // ReplicaSet owns Pod (ownership chain)
	"SCHEDULED_ON":     EdgeCategoryMaterialization, // Pod scheduled on Node
	"SELECTS":          EdgeCategoryMaterialization, // Service/NetworkPolicy selects Pod
	"MOUNTS":           EdgeCategoryMaterialization, // Pod mounts ConfigMap/Secret/PVC
	"CREATES_OBSERVED": EdgeCategoryMaterialization, // Observed creation correlation
}

// ClassifyEdge returns the edge category for a relationship type
func ClassifyEdge(relationshipType string) string {
	if category, ok := edgeClassification[relationshipType]; ok {
		return category
	}
	// Default to materialization for unknown edges (conservative approach)
	return EdgeCategoryMaterialization
}

// GetCausalWeight returns the weight for ranking purposes
// Cause-introducing edges count toward effective causal distance
// Materialization edges do not count
func GetCausalWeight(edgeCategory string) float64 {
	if edgeCategory == EdgeCategoryCauseIntroducing {
		return 1.0
	}
	return 0.0
}

// causeIntroducingAnomalyTypes are anomaly types that can introduce failures
// These represent changes or conditions that cause downstream problems
var causeIntroducingAnomalyTypes = map[string]bool{
	// Change anomalies - configuration mutations
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

	// State anomalies - resource-level issues that cause downstream failures
	"NodeNotReady":        true,
	"DiskPressure":        true,
	"NodeDiskPressure":    true, // Legacy name (deprecated)
	"NodeMemoryPressure":  true,
	"NodePIDPressure":     true,
	"NodeNetworkPressure": true,
	"Evicted":             true, // Pod eviction can cause cascading failures (Job recreation, etc.)

	// Configuration anomalies - missing or invalid configuration that causes failures
	"SecretMissing":          true, // Missing Secret/ConfigMap reference
	"InvalidConfigReference": true, // Invalid config reference (FailedMount due to missing resource)
	"CertExpired":            true, // Certificate has expired

	// RBAC anomalies - permission issues that cause downstream failures
	"ServiceAccountMissing":      true, // Missing ServiceAccount reference
	"RBACDenied":                 true, // Forbidden event due to RBAC
	"ClusterRoleModified":        true, // ClusterRole spec changes
	"ClusterRoleBindingModified": true, // ClusterRoleBinding spec changes

	// Helm/GitOps anomalies - deployment pipeline issues that cause downstream failures
	"HelmUpgrade":         true, // HelmRelease version upgraded
	"HelmRollback":        true, // HelmRelease version rolled back
	"ValuesChanged":       true, // HelmRelease values configuration changed
	"HelmReleaseFailed":   true, // HelmRelease Ready=False or Released=False
	"KustomizationFailed": true, // Kustomization Ready=False

	// Image & registry anomalies - image issues that cause Pod failures
	"ImageNotFound":      true, // Image doesn't exist in registry
	"RegistryAuthFailed": true, // Registry authentication failed
	"ImagePullTimeout":   true, // Image pull timeout/connection error

	// Storage anomalies - volume/disk issues that cause Pod failures
	"PVCBindingFailed":   true, // PVC failed to bind to PV
	"VolumeMountFailed":  true, // Volume mount failed (non-config)
	"VolumeOutOfSpace":   true, // Disk/volume space exhaustion
	"ReadOnlyFilesystem": true, // Filesystem mounted/became read-only
}

// derivedFailureAnomalyTypes are anomaly types that are symptoms, not causes
// These represent the effects of upstream issues
var derivedFailureAnomalyTypes = map[string]bool{
	"CrashLoopBackOff": true, // Result of upstream issue
	// Note: ImagePullBackOff is context-dependent - see ClassifyImagePullAnomaly()
	// It's NOT listed here because it needs context-aware classification
	"OOMKilled":           true, // Result of resource limits
	"PodFailed":           true, // Result of upstream issue
	"PodPending":          true, // Result of scheduling/resources
	"ErrorStatus":         true, // Generic error state
	"TerminatingStatus":   true, // Deletion in progress
	"CreateContainerErr":  true, // Container creation failed
	"InitContainerFailed": true, // Init container failure (usually symptom of config issue)

	// Deployment/ReplicaSet derived failures - never stop traversal here
	"RolloutStuck":           true, // Deployment rollout stuck/ProgressDeadlineExceeded
	"ReplicaCreationFailure": true, // FailedCreate event on Deployment/ReplicaSet
}

// IsCauseIntroducingAnomaly checks if an anomaly type can introduce failures
func IsCauseIntroducingAnomaly(anomalyType string, category anomaly.AnomalyCategory) bool {
	// Change category anomalies are generally cause-introducing
	if category == anomaly.CategoryChange {
		return true
	}
	// Specific state anomalies are cause-introducing
	return causeIntroducingAnomalyTypes[anomalyType]
}

// IsDerivedFailureAnomaly checks if an anomaly type is a symptom/effect
func IsDerivedFailureAnomaly(anomalyType string) bool {
	return derivedFailureAnomalyTypes[anomalyType]
}

// HasCauseIntroducingAnomaly checks if a node has anomalies that could introduce failures
// Anomaly must occur BEFORE or AT the symptom's first failure time
func HasCauseIntroducingAnomaly(nodeAnomalies []anomaly.Anomaly, symptomFirstFailure time.Time) bool {
	for _, a := range nodeAnomalies {
		// Must occur BEFORE or AT symptom failure time
		if a.Timestamp.After(symptomFirstFailure) {
			continue
		}

		// Check if this is a cause-introducing anomaly type
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return true
		}
	}
	return false
}

// intermediateAnomalyTypes are cause-introducing anomalies that often represent
// intermediate effects rather than root causes. The causal path algorithm should
// continue past these to find deeper root causes.
var intermediateAnomalyTypes = map[string]bool{
	"ResourceCreated": true, // Often an intermediate effect of upstream changes (e.g., HelmRelease → Deployment)
}

// hasOnlyIntermediateCauseAnomalies checks if a node has only intermediate cause-introducing
// anomalies (like ResourceCreated). These nodes should be traversed past to find deeper root causes.
func hasOnlyIntermediateCauseAnomalies(nodeAnomalies []anomaly.Anomaly) bool {
	if len(nodeAnomalies) == 0 {
		return false // No anomalies = not intermediate
	}

	hasIntermediateCause := false
	for _, a := range nodeAnomalies {
		// If we find a non-intermediate cause-introducing anomaly, return false
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			if !intermediateAnomalyTypes[a.Type] {
				return false // Has definitive cause anomaly
			}
			hasIntermediateCause = true
		}
	}
	return hasIntermediateCause
}

// hasDefinitiveCauseIntroducingAnomaly checks if a node has a "definitive" cause anomaly
// HasOnlyDerivedAnomalies checks if a node has only derived failure anomalies
// Returns true if there are no cause-introducing anomalies
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

// GetFirstCauseIntroducingAnomaly returns the earliest cause-introducing anomaly
func GetFirstCauseIntroducingAnomaly(nodeAnomalies []anomaly.Anomaly, beforeTime time.Time) *anomaly.Anomaly {
	var earliest *anomaly.Anomaly

	for i := range nodeAnomalies {
		a := &nodeAnomalies[i]

		// Must occur before the reference time
		if a.Timestamp.After(beforeTime) {
			continue
		}

		// Must be cause-introducing
		if !IsCauseIntroducingAnomaly(a.Type, a.Category) {
			continue
		}

		// Track earliest
		if earliest == nil || a.Timestamp.Before(earliest.Timestamp) {
			earliest = a
		}
	}

	return earliest
}

// ClassifyImagePullAnomaly determines whether an ImagePullBackOff/anomalyTypeErrImagePull anomaly
// is cause-introducing (the image genuinely doesn't exist or auth failed) or derived
// (network issue, node problem, etc. causing the pull to fail).
//
// Returns the reclassified anomaly type:
// - "ImageNotFound" (cause-introducing): image doesn't exist in registry
// - "RegistryAuthFailed" (cause-introducing): authentication/authorization failed
// - "ImagePullTimeout" (cause-introducing): timeout/connection errors
// - "ImagePullBackOff" (derived): transient or unknown cause, continue traversal
func ClassifyImagePullAnomaly(a anomaly.Anomaly) string {
	if a.Type != reasonImagePullBackOff && a.Type != anomalyTypeErrImagePull {
		return a.Type // Not an image pull anomaly
	}

	msg := strings.ToLower(a.Summary)

	// Check for definitive cause: image doesn't exist
	if strings.Contains(msg, "manifest unknown") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "repository does not exist") ||
		strings.Contains(msg, "name unknown") {
		return "ImageNotFound" // Cause-introducing
	}

	// Check for definitive cause: authentication/authorization failure
	if strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "authentication required") ||
		strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "no basic auth credentials") ||
		strings.Contains(msg, "x509") {
		return "RegistryAuthFailed" // Cause-introducing
	}

	// Check for timeout/connection errors (potentially cause-introducing if registry is down)
	if strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network unreachable") {
		return "ImagePullTimeout" // Cause-introducing (registry/network issue)
	}

	// Default: treat as derived (transient error, continue traversal)
	return reasonImagePullBackOff
}

// IsContextuallyDerivedAnomaly checks if an anomaly should be treated as derived
// based on additional context (other anomalies in the graph, resource relationships).
// This handles cases where an anomaly type is sometimes cause-introducing and
// sometimes derived depending on context.
func IsContextuallyDerivedAnomaly(a anomaly.Anomaly, relatedAnomalies map[string][]anomaly.Anomaly) bool {
	switch a.Type {
	case reasonImagePullBackOff, anomalyTypeErrImagePull:
		// Reclassify based on message content
		reclassified := ClassifyImagePullAnomaly(a)
		// If reclassified to ImagePullBackOff (unchanged), it's derived
		return reclassified == reasonImagePullBackOff

	case "Evicted":
		// Evicted is derived if the Node has a pressure condition
		return isEvictedDerivedFromNodePressure(relatedAnomalies)

	default:
		return derivedFailureAnomalyTypes[a.Type]
	}
}

// isEvictedDerivedFromNodePressure checks if a Pod eviction is caused by Node pressure.
// If the Node has DiskPressure, MemoryPressure, PIDPressure, or NodeNotReady,
// then the Evicted anomaly is derived (the Node condition is the root cause).
func isEvictedDerivedFromNodePressure(relatedAnomalies map[string][]anomaly.Anomaly) bool {
	nodePressureTypes := map[string]bool{
		"DiskPressure":        true,
		"NodeDiskPressure":    true,
		"NodeMemoryPressure":  true,
		"MemoryPressure":      true,
		"NodePIDPressure":     true,
		"PIDPressure":         true,
		"NodeNetworkPressure": true,
		"NodeNotReady":        true,
	}

	// Check all related nodes for pressure conditions
	for _, anomalies := range relatedAnomalies {
		for _, a := range anomalies {
			if nodePressureTypes[a.Type] {
				// Found a Node pressure condition - Evicted is derived
				return true
			}
		}
	}

	// No Node pressure found - Evicted might be manual or unknown cause
	return false
}

// reconciliationEffectOnManagedWorkloads lists anomaly types that are typically
// effects of GitOps reconciliation rather than root causes. When a workload
// (Deployment, StatefulSet, etc.) is managed by HelmRelease/Kustomization,
// these anomalies on the workload are usually effects, not causes.
var reconciliationEffectOnManagedWorkloads = map[string]bool{
	"ImageChanged":          true, // Image change came from manager
	"SpecModified":          true, // Spec change came from manager
	"ResourceCreated":       true, // Resource was created by manager
	"EnvironmentChanged":    true, // Env vars changed by manager
	"ResourceLimitsChanged": true, // Limits changed by manager
	"WorkloadSpecModified":  true, // Workload spec changed by manager
}

// managedWorkloadKinds are Kubernetes resource types that can be managed by GitOps controllers
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

// IsReconciliationEffectAnomaly checks if an anomaly on a resource is likely
// an effect of GitOps reconciliation rather than a root cause. This should be
// used when the resource has an upstream manager (HelmRelease, Kustomization, etc.).
func IsReconciliationEffectAnomaly(anomalies []anomaly.Anomaly, resourceKind string, hasUpstreamManager bool) bool {
	// Only applies to managed workloads with an upstream manager
	if !managedWorkloadKinds[resourceKind] || !hasUpstreamManager {
		return false
	}

	// Check if ALL anomalies are reconciliation effects
	// If any anomaly is NOT a reconciliation effect, we should consider this node
	for _, a := range anomalies {
		if !reconciliationEffectOnManagedWorkloads[a.Type] {
			return false // Has a non-reconciliation anomaly
		}
	}

	// All anomalies are reconciliation effects - continue to manager
	return len(anomalies) > 0
}

// HasCauseIntroducingAnomalyWithContext is an enhanced version of HasCauseIntroducingAnomaly
// that considers context from related resources to properly classify context-dependent anomalies.
func HasCauseIntroducingAnomalyWithContext(
	nodeAnomalies []anomaly.Anomaly,
	relatedAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
) bool {
	// Check for upstream ImageChanged to contextualize ImagePullBackOff
	upstreamHasImageChanged := hasUpstreamImageChangedAnomaly(relatedAnomalies)

	for _, a := range nodeAnomalies {
		// Must occur BEFORE or AT symptom failure time
		if a.Timestamp.After(symptomFirstFailure) {
			continue
		}

		// Check if this is contextually derived
		if IsContextuallyDerivedAnomaly(a, relatedAnomalies) {
			continue
		}

		// Handle image pull anomalies with context-aware classification
		if a.Type == "ImagePullBackOff" || a.Type == anomalyTypeErrImagePull {
			// If upstream has ImageChanged, ImagePullBackOff is derived (symptom of the image change)
			if upstreamHasImageChanged {
				continue
			}
			reclassified := ClassifyImagePullAnomaly(a)
			if causeIntroducingAnomalyTypes[reclassified] {
				return true
			}
			continue
		}

		// Check if this is a cause-introducing anomaly type
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return true
		}
	}
	return false
}

// hasDefinitiveCauseIntroducingAnomalyWithContext is an enhanced version of hasDefinitiveCauseIntroducingAnomaly
// that considers context from related resources. This is critical for cases like:
// - Evicted pods: derived if Node has DiskPressure/MemoryPressure
// - ImagePullBackOff: derived if upstream has ImageChanged anomaly
func hasDefinitiveCauseIntroducingAnomalyWithContext(
	nodeAnomalies []anomaly.Anomaly,
	allNodeAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
) bool {
	// Check for upstream ImageChanged to contextualize ImagePullBackOff
	upstreamHasImageChanged := hasUpstreamImageChangedAnomaly(allNodeAnomalies)

	for _, a := range nodeAnomalies {
		// Must occur BEFORE or AT symptom failure time
		if a.Timestamp.After(symptomFirstFailure) {
			continue
		}

		// Skip intermediate anomaly types
		if intermediateAnomalyTypes[a.Type] {
			continue
		}

		// Check if this is contextually derived
		if IsContextuallyDerivedAnomaly(a, allNodeAnomalies) {
			continue
		}

		// Handle image pull anomalies with context-aware classification
		if a.Type == "ImagePullBackOff" || a.Type == anomalyTypeErrImagePull {
			// If upstream has ImageChanged, ImagePullBackOff is derived (symptom of the image change)
			if upstreamHasImageChanged {
				continue
			}
			reclassified := ClassifyImagePullAnomaly(a)
			if causeIntroducingAnomalyTypes[reclassified] {
				return true
			}
			continue
		}

		// Check if this is a cause-introducing anomaly type
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return true
		}
	}
	return false
}

// hasUpstreamImageChangedAnomaly checks if any node in the graph has an ImageChanged anomaly
// This is used to determine if ImagePullBackOff is derived (caused by image change) or primary
func hasUpstreamImageChangedAnomaly(allNodeAnomalies map[string][]anomaly.Anomaly) bool {
	for _, anomalies := range allNodeAnomalies {
		for _, a := range anomalies {
			if a.Type == "ImageChanged" {
				return true
			}
		}
	}
	return false
}

// intentOwnerKinds are Kubernetes resource types that represent "intent owners" -
// resources that directly encode user intent about desired state. These are preferred
// as root causes because they represent the source of truth for what should be deployed.
//
// Intent owners are:
// - GitOps controllers that define what should be deployed (HelmRelease, Kustomization, Application)
// - Infrastructure resources that affect workload scheduling (Node)
// - Configuration resources referenced by workloads (ConfigMap, Secret)
// - Storage resources that affect workload mounting (PersistentVolume, PersistentVolumeClaim)
// - RBAC resources that affect workload permissions (ClusterRole, Role, ClusterRoleBinding, RoleBinding)
var intentOwnerKinds = map[string]bool{
	// GitOps controllers - highest priority intent owners
	"HelmRelease":   true, // Flux Helm controller
	"Kustomization": true, // Flux Kustomize controller
	"Application":   true, // ArgoCD Application

	// Infrastructure resources
	"Node": true, // Node conditions directly affect Pod scheduling and eviction

	// Configuration resources
	"ConfigMap": true, // Application configuration
	"Secret":    true, // Sensitive configuration

	// Storage resources
	"PersistentVolume":      true, // Storage provisioning
	"PersistentVolumeClaim": true, // Storage requests

	// RBAC resources
	"ClusterRole":        true,
	"Role":               true,
	"ClusterRoleBinding": true,
	"RoleBinding":        true,
	"ServiceAccount":     true,
}

// gitOpsControllerKinds are the subset of intent owners that are GitOps controllers.
// These get an extra confidence boost because they represent the ultimate source of
// deployment intent in a GitOps workflow.
var gitOpsControllerKinds = map[string]bool{
	"HelmRelease":   true,
	"Kustomization": true,
	"Application":   true, // ArgoCD
}

// IsIntentOwner returns true if the given resource kind is an intent owner.
// Intent owners are preferred as root causes because they represent the source
// of truth for desired state.
func IsIntentOwner(kind string) bool {
	return intentOwnerKinds[kind]
}

// IsGitOpsController returns true if the given resource kind is a GitOps controller.
// GitOps controllers get an extra confidence boost as they represent the ultimate
// source of deployment intent.
func IsGitOpsController(kind string) bool {
	return gitOpsControllerKinds[kind]
}

// definitiveRootCauseAnomalyTypes are anomaly types that indicate the resource itself
// is the definitive root cause, not just an intermediate node in the causal chain.
// Resources with these anomalies should be ranked higher than downstream effects.
var definitiveRootCauseAnomalyTypes = map[string]bool{
	// Deletion anomalies - resource was removed
	"ResourceDeleted":       true,
	"Deleted":               true,
	"TerminatingStatus":     true,
	"SecretMissing":         true,
	"ConfigMapMissing":      true,
	"ServiceAccountMissing": true,

	// Node pressure anomalies - infrastructure-level root causes
	"NodeNotReady":        true,
	"DiskPressure":        true,
	"NodeDiskPressure":    true,
	"NodeMemoryPressure":  true,
	"NodePIDPressure":     true,
	"NodeNetworkPressure": true,

	// Configuration anomalies that are definitive causes
	"CertExpired":            true,
	"InvalidConfigReference": true,
}

// HasDefinitiveRootCauseAnomaly checks if a node has anomalies that indicate it's
// a definitive root cause (like ResourceDeleted) rather than an intermediate effect.
func HasDefinitiveRootCauseAnomaly(anomalies []anomaly.Anomaly) bool {
	for _, a := range anomalies {
		if definitiveRootCauseAnomalyTypes[a.Type] {
			return true
		}
	}
	return false
}
