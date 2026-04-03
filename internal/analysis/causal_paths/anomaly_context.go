package causalpaths

import (
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
)

const (
	reasonImagePullBackOff  = "ImagePullBackOff"
	anomalyTypeErrImagePull = "ErrImagePull"
)

// ClassifyImagePullAnomaly determines whether an ImagePullBackOff/ErrImagePull anomaly
// is cause-introducing (the image genuinely doesn't exist or auth failed) or derived
// (network issue, node problem, etc. causing the pull to fail).
func ClassifyImagePullAnomaly(a anomaly.Anomaly) string {
	if a.Type != reasonImagePullBackOff && a.Type != anomalyTypeErrImagePull {
		return a.Type
	}

	msg := strings.ToLower(a.Summary)

	if strings.Contains(msg, "manifest unknown") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "repository does not exist") ||
		strings.Contains(msg, "name unknown") {
		return "ImageNotFound"
	}

	if strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "authentication required") ||
		strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "no basic auth credentials") ||
		strings.Contains(msg, "x509") {
		return "RegistryAuthFailed"
	}

	if strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network unreachable") {
		return "ImagePullTimeout"
	}

	return reasonImagePullBackOff
}

// IsContextuallyDerivedAnomaly checks if an anomaly should be treated as derived
// based on additional context from related resources.
func IsContextuallyDerivedAnomaly(a anomaly.Anomaly, relatedAnomalies map[string][]anomaly.Anomaly) bool {
	switch a.Type {
	case reasonImagePullBackOff, anomalyTypeErrImagePull:
		return ClassifyImagePullAnomaly(a) == reasonImagePullBackOff
	case "Evicted":
		return isEvictedDerivedFromNodePressure(relatedAnomalies)
	default:
		return derivedFailureAnomalyTypes[a.Type]
	}
}

// isEvictedDerivedFromNodePressure checks if a Pod eviction is caused by Node pressure.
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

	for _, anomalies := range relatedAnomalies {
		for _, a := range anomalies {
			if nodePressureTypes[a.Type] {
				return true
			}
		}
	}

	return false
}

// HasCauseIntroducingAnomalyWithContext is an enhanced version of HasCauseIntroducingAnomaly
// that considers related resources to classify context-dependent anomalies.
func HasCauseIntroducingAnomalyWithContext(
	nodeAnomalies []anomaly.Anomaly,
	relatedAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
) bool {
	upstreamHasImageChanged := hasUpstreamImageChangedAnomaly(relatedAnomalies)

	for _, a := range nodeAnomalies {
		if a.Timestamp.After(symptomFirstFailure) {
			continue
		}
		if IsContextuallyDerivedAnomaly(a, relatedAnomalies) {
			continue
		}
		if a.Type == reasonImagePullBackOff || a.Type == anomalyTypeErrImagePull {
			if upstreamHasImageChanged {
				continue
			}
			reclassified := ClassifyImagePullAnomaly(a)
			if causeIntroducingAnomalyTypes[reclassified] {
				return true
			}
			continue
		}
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return true
		}
	}

	return false
}

// hasDefinitiveCauseIntroducingAnomalyWithContext is an enhanced version of
// hasDefinitiveCauseIntroducingAnomaly that considers related resources.
func hasDefinitiveCauseIntroducingAnomalyWithContext(
	nodeAnomalies []anomaly.Anomaly,
	allNodeAnomalies map[string][]anomaly.Anomaly,
	symptomFirstFailure time.Time,
) bool {
	upstreamHasImageChanged := hasUpstreamImageChangedAnomaly(allNodeAnomalies)

	for _, a := range nodeAnomalies {
		if a.Timestamp.After(symptomFirstFailure) {
			continue
		}
		if intermediateAnomalyTypes[a.Type] {
			continue
		}
		if IsContextuallyDerivedAnomaly(a, allNodeAnomalies) {
			continue
		}
		if a.Type == reasonImagePullBackOff || a.Type == anomalyTypeErrImagePull {
			if upstreamHasImageChanged {
				continue
			}
			reclassified := ClassifyImagePullAnomaly(a)
			if causeIntroducingAnomalyTypes[reclassified] {
				return true
			}
			continue
		}
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			return true
		}
	}

	return false
}

// hasUpstreamImageChangedAnomaly checks if any related node has an ImageChanged anomaly.
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
