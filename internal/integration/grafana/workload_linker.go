package grafana

// InferWorkloadFromLabels infers K8s workload from PromQL label selectors.
// Uses label priority: deployment > app.kubernetes.io/name > app > service > job > pod
//
// Key behaviors:
// - Namespace label sets confidence to 0.9 (high confidence for explicit namespace)
// - Workload name follows priority order (deployment highest, pod lowest)
// - Returns nil if no workload labels found (unlinked signal)
// - Tracks which label was used in InferredFrom field
// - Confidence varies by label type (deployment=0.9, app=0.7, pod=0.6)
//
// Returns:
// - *WorkloadInference: Inferred workload with namespace, name, source label, confidence
// - nil: No workload inference possible (empty labels or no workload labels)
func InferWorkloadFromLabels(labelSelectors map[string]string) *WorkloadInference {
	if len(labelSelectors) == 0 {
		return nil
	}

	// Extract namespace first
	namespace, hasNamespace := labelSelectors["namespace"]

	// Workload label priority order (highest to lowest confidence)
	// Each label type has associated confidence score
	type labelPriority struct {
		label      string
		confidence float64
	}

	priorities := []labelPriority{
		{"deployment", 0.9},                 // K8s Deployment label (explicit)
		{"app.kubernetes.io/name", 0.85},    // K8s recommended label
		{"app", 0.7},                        // Common convention
		{"service", 0.75},                   // Service label
		{"job", 0.8},                        // Job label (batch workloads)
		{"pod", 0.6},                        // Pod label (lowest priority)
	}

	// Try each label in priority order
	for _, priority := range priorities {
		if workloadName, exists := labelSelectors[priority.label]; exists && workloadName != "" {
			// Found workload label - create inference
			confidence := priority.confidence
			if hasNamespace {
				// Boost confidence if namespace is present
				confidence = 0.9
			}

			return &WorkloadInference{
				Namespace:    namespace,
				WorkloadName: workloadName,
				InferredFrom: priority.label,
				Confidence:   confidence,
			}
		}
	}

	// No workload labels found
	// If namespace exists, return inference with empty workload (namespace-only signal)
	// Otherwise return nil (completely unlinked signal)
	if hasNamespace {
		return &WorkloadInference{
			Namespace:    namespace,
			WorkloadName: "",
			InferredFrom: "namespace",
			Confidence:   0.7, // Lower confidence for namespace-only inference
		}
	}

	return nil
}
