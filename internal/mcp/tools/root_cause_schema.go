package tools

import "time"

// ============================================================================
// NEW CAUSALITY-FIRST SCHEMA
// ============================================================================
// This schema explicitly encodes causal reasoning derived from the graph,
// removing the need for LLMs to infer causality from symptoms.

// RootCauseAnalysisV2 represents the causality-first response schema
type RootCauseAnalysisV2 struct {
	Incident              IncidentAnalysis       `json:"incident"`
	SupportingEvidence    []EvidenceItem         `json:"supportingEvidence"`
	ExcludedAlternatives  []ExcludedHypothesis   `json:"excludedAlternatives,omitempty"`
	QueryMetadata         QueryMetadata          `json:"queryMetadata"`
}

// IncidentAnalysis contains the core causal reasoning
type IncidentAnalysis struct {
	ObservedSymptom ObservedSymptom     `json:"observedSymptom"`
	CausalChain     []CausalStep        `json:"causalChain"`
	RootCause       RootCauseHypothesis `json:"rootCause"`
	Confidence      ConfidenceScore     `json:"confidence"`
}

// ObservedSymptom contains only directly observed facts (no inference)
type ObservedSymptom struct {
	Resource      SymptomResource `json:"resource"`
	Status        string          `json:"status"`        // e.g., "Error", "CrashLoopBackOff"
	ErrorMessage  string          `json:"errorMessage"`  // Raw error from Kubernetes
	ObservedAt    time.Time       `json:"observedAt"`    // When the symptom was observed
	SymptomType   string          `json:"symptomType"`   // e.g., "ImagePullError", "CrashLoop", "OOMKilled"
}

// SymptomResource identifies the resource exhibiting the symptom
type SymptomResource struct {
	UID       string `json:"uid"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// CausalStep represents one hop in the causal chain
// Must be ordered from symptom → root cause
type CausalStep struct {
	StepNumber       int              `json:"stepNumber"`       // 1-indexed position in chain
	Resource         SymptomResource  `json:"resource"`
	ChangeEvent      *ChangeEventInfo `json:"changeEvent,omitempty"` // Event that occurred at this step
	RelationshipType string           `json:"relationshipType"` // "OWNS", "MANAGES", "TRIGGERED_BY", etc.
	RelationshipTo   *SymptomResource `json:"relationshipTo,omitempty"` // Next resource in chain
	Reasoning        string           `json:"reasoning"`        // Why this step is part of the causal chain
}

// ChangeEventInfo represents a change event in the causal chain
type ChangeEventInfo struct {
	EventID       string    `json:"eventId"`
	Timestamp     time.Time `json:"timestamp"`
	EventType     string    `json:"eventType"` // CREATE, UPDATE, DELETE
	ConfigChanged bool      `json:"configChanged,omitempty"`
	StatusChanged bool      `json:"statusChanged,omitempty"`
	Description   string    `json:"description,omitempty"` // Human-readable summary
}

// RootCauseHypothesis identifies the most likely root cause
type RootCauseHypothesis struct {
	Resource     SymptomResource  `json:"resource"`
	ChangeEvent  ChangeEventInfo  `json:"changeEvent"`
	CausationType string          `json:"causationType"` // "ConfigChange", "DeploymentUpdate", "ResourceScaling", etc.
	Explanation  string           `json:"explanation"`   // Why this change plausibly caused the symptom
	TimeLagMs    int64            `json:"timeLagMs"`     // Time between root cause and symptom
}

// ConfidenceScore with deterministic computation and rationale
type ConfidenceScore struct {
	Score     float64          `json:"score"`     // 0.0-1.0, deterministically computed
	Rationale string           `json:"rationale"` // Human-readable explanation of score
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
	Type        string                 `json:"type"`        // "RELATIONSHIP", "TEMPORAL", "ERROR_CORRELATION", etc.
	Description string                 `json:"description"` // Human-readable evidence
	Confidence  float64                `json:"confidence"`  // 0.0-1.0: Strength of this evidence
	Details     map[string]interface{} `json:"details,omitempty"`
}

// ExcludedHypothesis represents a plausible but rejected alternative
type ExcludedHypothesis struct {
	Resource      SymptomResource `json:"resource"`
	Hypothesis    string          `json:"hypothesis"`    // What was considered
	ReasonExcluded string         `json:"reasonExcluded"` // Why it was rejected
}

// QueryMetadata provides execution information
type QueryMetadata struct {
	QueryExecutionMs int64     `json:"queryExecutionMs"`
	GraphNodesVisited int      `json:"graphNodesVisited,omitempty"`
	AlgorithmVersion string    `json:"algorithmVersion"` // For reproducibility
	ExecutedAt       time.Time `json:"executedAt"`
}
