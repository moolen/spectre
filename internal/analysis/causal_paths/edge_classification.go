package causalpaths

// edgeClassification maps relationship types to their causal category.
// Cause-introducing edges represent relationships where changes propagate causally.
// Materialization edges represent structural/ownership relationships.
var edgeClassification = map[string]string{
	// Cause-introducing edges (changes propagate causally).
	"MANAGES":         EdgeCategoryCauseIntroducing, // HelmRelease/Kustomization manages lifecycle (special direction handling in buildUpstreamAdjacency)
	"TRIGGERED_BY":    EdgeCategoryCauseIntroducing, // Explicit causal trigger
	"REFERENCES_SPEC": EdgeCategoryCauseIntroducing, // ConfigMap/Secret reference in spec

	// RBAC edges are cause-introducing because permission changes propagate to Pods.
	// Direction: RoleBinding --GRANTS_TO--> ServiceAccount, RoleBinding --BINDS_ROLE--> Role.
	// Causal flow: Role change -> affects RoleBinding -> affects ServiceAccount permissions -> Pod fails.
	"USES_SERVICE_ACCOUNT": EdgeCategoryCauseIntroducing, // Pod uses ServiceAccount (SA permissions affect Pod)
	"GRANTS_TO":            EdgeCategoryCauseIntroducing, // RoleBinding grants to ServiceAccount (special direction handling in buildUpstreamAdjacency)
	"BINDS_ROLE":           EdgeCategoryCauseIntroducing, // RoleBinding binds Role

	// Materialization edges (structural/scheduling relationships).
	"OWNS":             EdgeCategoryMaterialization, // ReplicaSet owns Pod (ownership chain)
	"SCHEDULED_ON":     EdgeCategoryMaterialization, // Pod scheduled on Node
	"SELECTS":          EdgeCategoryMaterialization, // Service/NetworkPolicy selects Pod
	"MOUNTS":           EdgeCategoryMaterialization, // Pod mounts ConfigMap/Secret/PVC
	"CREATES_OBSERVED": EdgeCategoryMaterialization, // Observed creation correlation
}

// ClassifyEdge returns the edge category for a relationship type.
func ClassifyEdge(relationshipType string) string {
	if category, ok := edgeClassification[relationshipType]; ok {
		return category
	}

	// Default to materialization for unknown edges (conservative approach).
	return EdgeCategoryMaterialization
}

// GetCausalWeight returns the weight for ranking purposes.
// Cause-introducing edges count toward effective causal distance.
// Materialization edges do not count.
func GetCausalWeight(edgeCategory string) float64 {
	if edgeCategory == EdgeCategoryCauseIntroducing {
		return 1.0
	}

	return 0.0
}
