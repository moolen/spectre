package analysis

import (
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// ============================================================================
// NEW CAUSALITY-FIRST SCHEMA
// ============================================================================
// This schema explicitly encodes causal reasoning derived from the graph,
// removing the need for LLMs to infer causality from symptoms.

// RootCauseAnalysisV2 represents the causality-first response schema
type RootCauseAnalysisV2 struct {
	Incident             IncidentAnalysis     `json:"incident"`
	SupportingEvidence   []EvidenceItem       `json:"supportingEvidence"`
	ExcludedAlternatives []ExcludedHypothesis `json:"excludedAlternatives,omitempty"`
	QueryMetadata        QueryMetadata        `json:"queryMetadata"`
}

// IncidentAnalysis contains the core causal reasoning
type IncidentAnalysis struct {
	ObservedSymptom ObservedSymptom     `json:"observedSymptom"`
	Graph           CausalGraph         `json:"graph"`
	RootCause       RootCauseHypothesis `json:"rootCause"`
	Confidence      ConfidenceScore     `json:"confidence"`
}

// ObservedSymptom contains only directly observed facts (no inference)
type ObservedSymptom struct {
	Resource     SymptomResource `json:"resource"`
	Status       string          `json:"status"`       // e.g., "Error", "CrashLoopBackOff"
	ErrorMessage string          `json:"errorMessage"` // Raw error from Kubernetes
	ObservedAt   time.Time       `json:"observedAt"`   // When the symptom was observed
	SymptomType  string          `json:"symptomType"`  // e.g., "ImagePullError", "CrashLoop", "OOMKilled"
}

// SymptomResource identifies the resource exhibiting the symptom
type SymptomResource struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// GraphNode represents any resource in the causal graph
type GraphNode struct {
	ID          string            `json:"id"`                   // Unique identifier (e.g., "node-{uid}")
	Resource    SymptomResource   `json:"resource"`
	ChangeEvent *ChangeEventInfo  `json:"changeEvent,omitempty"`  // Primary event (highest priority)
	AllEvents   []ChangeEventInfo `json:"allEvents,omitempty"`    // All events for this resource
	K8sEvents   []K8sEventInfo    `json:"k8sEvents,omitempty"`    // Kubernetes Events
	NodeType    string            `json:"nodeType"`               // "SPINE" or "RELATED"
	StepNumber  int               `json:"stepNumber,omitempty"`   // For SPINE nodes (ordering)
	Reasoning   string            `json:"reasoning,omitempty"`    // Why this node is in the graph
}

// GraphEdge represents a relationship between resources
type GraphEdge struct {
	ID               string `json:"id"`               // Unique identifier
	From             string `json:"from"`             // Source node ID
	To               string `json:"to"`               // Target node ID
	RelationshipType string `json:"relationshipType"` // OWNS, MANAGES, SCHEDULED_ON, etc.
	EdgeType         string `json:"edgeType"`         // "SPINE" or "ATTACHMENT"
}

// CausalGraph represents the incident as a graph
type CausalGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// ChangeEventInfo represents a change event in the causal chain
type ChangeEventInfo struct {
	EventID       string    `json:"eventId"`
	Timestamp     time.Time `json:"timestamp"`
	EventType     string    `json:"eventType"` // CREATE, UPDATE, DELETE
	Status        string    `json:"status,omitempty"` // Inferred resource status: Ready, Warning, Error, Terminating, Unknown
	ConfigChanged bool      `json:"configChanged,omitempty"`
	StatusChanged bool      `json:"statusChanged,omitempty"`
	Description   string    `json:"description,omitempty"` // Human-readable summary
	Data          []byte    `json:"data,omitempty"`        // Full resource JSON for diff
}

// K8sEventInfo represents a Kubernetes Event (kind: Event) related to a resource
type K8sEventInfo struct {
	EventID   string    `json:"eventId"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`  // e.g., "FailedScheduling", "BackOff"
	Message   string    `json:"message"` // Human-readable message
	Type      string    `json:"type"`    // "Warning", "Normal", "Error"
	Count     int       `json:"count"`   // How many times this event occurred
	Source    string    `json:"source"`  // Component that generated the event
}

// RootCauseHypothesis identifies the most likely root cause
type RootCauseHypothesis struct {
	Resource      SymptomResource `json:"resource"`
	ChangeEvent   ChangeEventInfo `json:"changeEvent"`
	CausationType string          `json:"causationType"` // "ConfigChange", "DeploymentUpdate", "ResourceScaling", etc.
	Explanation   string          `json:"explanation"`   // Why this change plausibly caused the symptom
	TimeLagMs     int64           `json:"timeLagMs"`     // Time between root cause and symptom
}

// ConfidenceScore with deterministic computation and rationale
type ConfidenceScore struct {
	Score     float64           `json:"score"`     // 0.0-1.0, deterministically computed
	Rationale string            `json:"rationale"` // Human-readable explanation of score
	Factors   ConfidenceFactors `json:"factors"`   // Breakdown of contributing factors
}

// ConfidenceFactors breaks down the confidence calculation
type ConfidenceFactors struct {
	DirectSpecChange     float64 `json:"directSpecChange"`     // 0.0-1.0: Did spec change?
	TemporalProximity    float64 `json:"temporalProximity"`    // 0.0-1.0: How close in time?
	RelationshipStrength float64 `json:"relationshipStrength"` // 0.0-1.0: MANAGES=1.0, OWNS=0.8, etc.
	ErrorMessageMatch    float64 `json:"errorMessageMatch"`    // 0.0-1.0: Does error explain symptom?
	ChainCompleteness    float64 `json:"chainCompleteness"`    // 0.0-1.0: How complete is the chain?
}

// EvidenceItem represents supporting evidence for the root cause
type EvidenceItem struct {
	Type        string                 `json:"type"`              // "RELATIONSHIP", "TEMPORAL", "ERROR_CORRELATION", etc.
	Description string                 `json:"description"`       // Human-readable evidence
	Confidence  float64                `json:"confidence"`        // 0.0-1.0: Strength of this evidence
	Details     map[string]interface{} `json:"details,omitempty"`
}

// ExcludedHypothesis represents a plausible but rejected alternative
type ExcludedHypothesis struct {
	Resource       SymptomResource `json:"resource"`
	Hypothesis     string          `json:"hypothesis"`     // What was considered
	ReasonExcluded string          `json:"reasonExcluded"` // Why it was rejected
}

// QueryMetadata provides execution information and result quality indicators
type QueryMetadata struct {
	QueryExecutionMs  int64           `json:"queryExecutionMs"`
	GraphNodesVisited int             `json:"graphNodesVisited,omitempty"`
	AlgorithmVersion  string          `json:"algorithmVersion"` // For reproducibility
	ExecutedAt        time.Time       `json:"executedAt"`
	ResultQuality     ResultQuality   `json:"resultQuality"`           // Quality indicators for this result
	PerformanceMetrics *PerformanceMetrics `json:"performanceMetrics,omitempty"` // Detailed timing breakdown
}

// ResultQuality indicates the quality and completeness of the analysis result
type ResultQuality struct {
	// IsDegraded indicates if this is a degraded result due to errors or missing data.
	// Degraded results should be treated with caution and may not include full causality.
	IsDegraded bool `json:"isDegraded"`

	// DegradationReasons lists specific reasons why the result was degraded.
	// Examples: "ownership_chain_query_failed", "no_events_found", "empty_graph"
	DegradationReasons []string `json:"degradationReasons,omitempty"`

	// IsSymptomOnly indicates if the result only contains the symptom resource
	// without any causal chain. This typically means no ownership chain was found.
	IsSymptomOnly bool `json:"isSymptomOnly"`

	// HasPartialData indicates if some optional data is missing but core analysis succeeded.
	// For example, missing K8s events or related resources.
	HasPartialData bool `json:"hasPartialData,omitempty"`

	// Warnings contains non-fatal issues encountered during analysis.
	// The result is still valid but these issues should be noted.
	Warnings []string `json:"warnings,omitempty"`
}

// PerformanceMetrics provides detailed timing breakdown for performance monitoring
type PerformanceMetrics struct {
	// TotalDurationMs is the total time for the entire analysis
	TotalDurationMs int64 `json:"totalDurationMs"`

	// GraphQueryDurationMs is the time spent executing all graph queries
	GraphQueryDurationMs int64 `json:"graphQueryDurationMs,omitempty"`

	// GraphBuildDurationMs is the time spent building the causal graph structure
	GraphBuildDurationMs int64 `json:"graphBuildDurationMs,omitempty"`

	// OwnershipChainDurationMs is the time spent querying the ownership chain
	OwnershipChainDurationMs int64 `json:"ownershipChainDurationMs,omitempty"`

	// SlowOperations lists operations that exceeded performance thresholds
	SlowOperations []SlowOperation `json:"slowOperations,omitempty"`
}

// SlowOperation records an operation that exceeded its performance threshold
type SlowOperation struct {
	Operation   string `json:"operation"`   // Name of the operation (e.g., "ownership_chain_query")
	DurationMs  int64  `json:"durationMs"`  // Actual duration
	ThresholdMs int64  `json:"thresholdMs"` // Expected threshold
}

// ============================================================================
// GRAPH QUERY TYPES
// ============================================================================
// These types represent intermediate data structures used during graph queries
// to build the causal chain.

// ResourceWithDistance represents a resource in the ownership chain with its distance from the symptom
type ResourceWithDistance struct {
	Resource graph.ResourceIdentity
	Distance int
}

// ManagerData contains manager information for a resource
type ManagerData struct {
	Manager     graph.ResourceIdentity
	ManagesEdge graph.ManagesEdge
}

// RelatedResourceData contains information about a related resource
type RelatedResourceData struct {
	Resource           graph.ResourceIdentity
	RelationshipType   string
	Events             []ChangeEventInfo
	ReferenceTargetUID string // For INGRESS_REF, the UID of the Service that the Ingress references
}
