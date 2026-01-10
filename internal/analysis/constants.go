package analysis

import "time"

// ============================================================================
// QUERY CONFIGURATION
// ============================================================================

const (
	// DefaultQueryTimeoutMs is the default timeout for graph queries in milliseconds.
	// Set to 120 seconds to accommodate resource-constrained environments.
	DefaultQueryTimeoutMs = 120000

	// DefaultLookbackWindow is the default time window to look back for related events.
	// 10 minutes is chosen as a reasonable balance between finding relevant changes
	// and avoiding noise from older unrelated events.
	DefaultLookbackWindow = 10 * time.Minute

	// DefaultLookbackNs is DefaultLookbackWindow in nanoseconds for internal use.
	DefaultLookbackNs = int64(DefaultLookbackWindow)
)

// ============================================================================
// GRAPH TRAVERSAL LIMITS
// ============================================================================

const (
	// MaxOwnershipDepth is the maximum depth to traverse ownership chains.
	// 3 levels covers typical scenarios: Pod <- ReplicaSet <- Deployment <- HelmRelease
	MaxOwnershipDepth = 3

	// MaxRecentEvents is the maximum number of recent events to include per resource.
	// This prevents overwhelming the response with too many events while ensuring
	// we capture recent context.
	MaxRecentEvents = 10

	// MaxK8sEvents is the maximum number of Kubernetes events to include per resource.
	// K8s events can be very noisy, so we limit to the most recent ones.
	MaxK8sEvents = 20
)

// ============================================================================
// CONFIDENCE SCORING
// ============================================================================

const (
	// MinManagerConfidence is the minimum confidence score for MANAGES edges.
	// Manager relationships below this threshold are filtered out as they're
	// likely to be noise or weak associations.
	MinManagerConfidence = 0.5

	// ConfidenceWeightSpecChange is the weight for spec change factor in confidence calculation.
	// Spec changes are the most important signal for root cause analysis.
	ConfidenceWeightSpecChange = 0.35

	// ConfidenceWeightTemporal is the weight for temporal proximity in confidence calculation.
	// How close in time the change occurred to the symptom.
	ConfidenceWeightTemporal = 0.30

	// ConfidenceWeightRelationship is the weight for relationship strength in confidence calculation.
	// MANAGES relationships are stronger than OWNS relationships.
	ConfidenceWeightRelationship = 0.20

	// ConfidenceWeightChain is the weight for chain completeness in confidence calculation.
	// More complete chains indicate better understanding of the causality.
	ConfidenceWeightChain = 0.10

	// ConfidenceWeightErrorMatch is the weight for error message matching in confidence calculation.
	// This is the least important factor as error messages can be misleading.
	ConfidenceWeightErrorMatch = 0.05

	// TemporalFactorMaxLag is the maximum lag in milliseconds for temporal factor calculation.
	// Changes beyond this time are considered unlikely to be the root cause.
	// 10 minutes = 600,000ms
	TemporalFactorMaxLag = int64(10 * time.Minute / time.Millisecond)

	// RelationshipStrengthManages is the strength score for MANAGES relationships.
	RelationshipStrengthManages = 1.0

	// RelationshipStrengthOwns is the strength score for OWNS relationships.
	RelationshipStrengthOwns = 0.8

	// RelationshipStrengthTriggeredBy is the strength score for TRIGGERED_BY relationships.
	RelationshipStrengthTriggeredBy = 0.7

	// RelationshipStrengthDefault is the default strength score for unknown relationships.
	RelationshipStrengthDefault = 0.5

	// SpecChangeFactorUpdate is the factor for UPDATE events without config change.
	// Status-only updates have less confidence than true configuration changes.
	SpecChangeFactorUpdate = 0.5

	// ErrorMatchFactorGeneric is the factor for generic error messages.
	// Generic errors provide some signal but are not as strong as specific errors.
	ErrorMatchFactorGeneric = 0.5

	// ChainCompletenessMinNodes is the minimum number of SPINE nodes for full completeness.
	// A chain with at least this many nodes is considered complete.
	ChainCompletenessMinNodes = 3
)

// ============================================================================
// PERFORMANCE THRESHOLDS
// ============================================================================

const (
	// SlowQueryThresholdMs is the threshold for logging slow queries.
	// Queries exceeding this duration will be logged as warnings.
	SlowQueryThresholdMs = 5000

	// SlowGraphBuildThresholdMs is the threshold for logging slow graph building.
	// Graph building exceeding this duration will be logged as warnings.
	SlowGraphBuildThresholdMs = 10000

	// SlowAnalysisThresholdMs is the threshold for logging slow analysis.
	// Full analysis exceeding this duration will be logged as warnings.
	SlowAnalysisThresholdMs = 20000
)
