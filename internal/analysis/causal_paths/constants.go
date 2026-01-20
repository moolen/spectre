package causalpaths

import "time"

// Algorithm version for reproducibility tracking
const AlgorithmVersion = "v1.0-deterministic"

// Default input parameters
const (
	DefaultLookbackNs = int64(10 * time.Minute) // 10 minute lookback window
	DefaultMaxDepth   = 5                       // Maximum traversal depth
	DefaultMaxPaths   = 5                       // Maximum paths to return
)

// Input validation limits
const (
	MinMaxDepth = 1
	MaxMaxDepth = 10
	MinMaxPaths = 1
	MaxMaxPaths = 20
)

// Ranking weights (sum to 1.0)
const (
	WeightTemporal = 0.40 // Temporal proximity is most important
	WeightDistance = 0.35 // Shorter causal distance preferred
	WeightSeverity = 0.25 // Higher severity anomalies preferred
)

// Temporal scoring constants
const (
	MaxTemporalLagNs = int64(10 * time.Minute) // Max lookback window for temporal scoring
)

// Distance normalization
const (
	MaxEffectiveCausalDistance = 5 // Normalize distances to this max
)

// Intent owner confidence boosts
// These are additive bonuses applied to paths ending at intent owners
const (
	// IntentOwnerBoost is applied to paths ending at known intent owners
	// (ConfigMap, Secret, Node, RBAC resources, etc.)
	IntentOwnerBoost = 0.05

	// GitOpsControllerBoost is an additional boost for GitOps controllers
	// (HelmRelease, Kustomization, ArgoCD Application)
	// Total boost for GitOps controllers = IntentOwnerBoost + GitOpsControllerBoost = 0.10
	GitOpsControllerBoost = 0.05

	// DefinitiveRootCauseBoost is applied when the root cause has a "definitive" anomaly
	// like ResourceDeleted, which indicates the resource itself is the actual root cause
	// rather than being a symptom of another issue. This should outweigh GitOps boosts
	// when a deleted ConfigMap/Secret causes a HelmRelease to fail.
	DefinitiveRootCauseBoost = 0.15
)

// Edge categories
const (
	EdgeCategoryCauseIntroducing = "CAUSE_INTRODUCING"
	EdgeCategoryMaterialization  = "MATERIALIZATION"
)
