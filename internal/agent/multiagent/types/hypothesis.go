// Package types defines the core data structures for the multi-agent incident response system.
package types

import "time"

// Hypothesis represents a root-cause hypothesis following the mandatory schema.
// This is the primary output of the hypothesis building pipeline and must be
// validated by the IncidentReviewerAgent before being presented to users.
type Hypothesis struct {
	// ID is a unique identifier for this hypothesis within the investigation.
	ID string `json:"id"`

	// Claim is a clear, falsifiable statement of what is believed to be the root cause.
	// Good: "The payment-service errors are caused by the ConfigMap update at 10:03 that changed DB_CONNECTION_STRING"
	// Bad: "Something is wrong with the configuration"
	Claim string `json:"claim"`

	// SupportingEvidence links this hypothesis to specific data from the SystemSnapshot.
	SupportingEvidence []EvidenceRef `json:"supporting_evidence"`

	// Assumptions lists all explicit and implicit assumptions underlying this hypothesis.
	Assumptions []Assumption `json:"assumptions"`

	// ValidationPlan defines how to confirm or falsify this hypothesis.
	ValidationPlan ValidationPlan `json:"validation_plan"`

	// Confidence is a calibrated probability score from 0.0 to 1.0.
	// For MVP, this is capped at 0.85 to prevent overconfidence.
	// Guidelines:
	//   0.70-0.85: Strong evidence, tight temporal correlation
	//   0.50-0.70: Moderate evidence, plausible but uncertain
	//   0.30-0.50: Weak evidence, one of several possibilities
	//   <0.30: Speculative, minimal supporting data
	Confidence float64 `json:"confidence"`

	// Status indicates the review status of this hypothesis.
	Status HypothesisStatus `json:"status"`

	// RejectionReason is set when Status is HypothesisStatusRejected.
	// This is visible to users to explain why the hypothesis was rejected.
	RejectionReason string `json:"rejection_reason,omitempty"`

	// CreatedAt is when this hypothesis was generated.
	CreatedAt time.Time `json:"created_at"`
}

// EvidenceRef links a hypothesis to supporting data from the SystemSnapshot.
type EvidenceRef struct {
	// Type categorizes the kind of evidence.
	Type EvidenceType `json:"type"`

	// SourceID is a reference to a specific item in the SystemSnapshot.
	// Format: "<snapshot_field>/<item_index>" or "<snapshot_field>/<unique_id>"
	// Examples: "causal_paths/0", "anomalies/abc123", "recent_changes/2"
	SourceID string `json:"source_id"`

	// Description explains what this evidence shows in relation to the claim.
	Description string `json:"description"`

	// Strength indicates how strongly this evidence supports the claim.
	Strength EvidenceStrength `json:"strength"`
}

// EvidenceType categorizes the kind of evidence from the SystemSnapshot.
type EvidenceType string

const (
	EvidenceTypeCausalPath    EvidenceType = "causal_path"
	EvidenceTypeAnomaly       EvidenceType = "anomaly"
	EvidenceTypeChange        EvidenceType = "change"
	EvidenceTypeEvent         EvidenceType = "event"
	EvidenceTypeResourceState EvidenceType = "resource_state"
	EvidenceTypeClusterHealth EvidenceType = "cluster_health"
)

// EvidenceStrength indicates how strongly evidence supports a claim.
type EvidenceStrength string

const (
	EvidenceStrengthStrong   EvidenceStrength = "strong"
	EvidenceStrengthModerate EvidenceStrength = "moderate"
	EvidenceStrengthWeak     EvidenceStrength = "weak"
)

// Assumption represents an explicit or implicit assumption in a hypothesis.
// All assumptions must be surfaced to prevent hidden reasoning.
type Assumption struct {
	// Description is a clear statement of the assumption.
	Description string `json:"description"`

	// IsVerified indicates whether this assumption has been verified.
	IsVerified bool `json:"is_verified"`

	// Falsifiable indicates whether this assumption can be disproven.
	Falsifiable bool `json:"falsifiable"`

	// FalsificationMethod describes how to disprove this assumption.
	// Required if Falsifiable is true.
	FalsificationMethod string `json:"falsification_method,omitempty"`
}

// ValidationPlan defines how to confirm or falsify a hypothesis.
type ValidationPlan struct {
	// ConfirmationChecks are tests that would support the hypothesis if they pass.
	ConfirmationChecks []ValidationTask `json:"confirmation_checks"`

	// FalsificationChecks are tests that would disprove the hypothesis if they pass.
	// At least one falsification check is required for a valid hypothesis.
	FalsificationChecks []ValidationTask `json:"falsification_checks"`

	// AdditionalDataNeeded lists information gaps that would help evaluate this hypothesis.
	AdditionalDataNeeded []string `json:"additional_data_needed,omitempty"`
}

// ValidationTask describes a specific check to perform.
type ValidationTask struct {
	// Description is a human-readable explanation of what to check.
	Description string `json:"description"`

	// Tool is the Spectre tool to use for this check (optional).
	Tool string `json:"tool,omitempty"`

	// Command is a kubectl or other CLI command suggestion (optional).
	Command string `json:"command,omitempty"`

	// Expected describes the expected result if the hypothesis is true/false.
	Expected string `json:"expected"`
}

// HypothesisStatus indicates the review status of a hypothesis.
type HypothesisStatus string

const (
	// HypothesisStatusPending indicates the hypothesis has not yet been reviewed.
	HypothesisStatusPending HypothesisStatus = "pending"

	// HypothesisStatusApproved indicates the hypothesis passed review without changes.
	HypothesisStatusApproved HypothesisStatus = "approved"

	// HypothesisStatusModified indicates the hypothesis was approved with changes.
	HypothesisStatusModified HypothesisStatus = "modified"

	// HypothesisStatusRejected indicates the hypothesis failed review.
	// The RejectionReason field will explain why.
	// Rejected hypotheses are visible to users with their rejection reason.
	HypothesisStatusRejected HypothesisStatus = "rejected"
)

// ReviewedHypotheses is the output of the IncidentReviewerAgent.
type ReviewedHypotheses struct {
	// Hypotheses contains all hypotheses with their updated status.
	// This includes approved, modified, and rejected hypotheses.
	Hypotheses []Hypothesis `json:"hypotheses"`

	// ReviewNotes is an overall summary of the review process.
	ReviewNotes string `json:"review_notes"`

	// Modifications lists specific changes made to hypotheses.
	Modifications []Modification `json:"modifications,omitempty"`
}

// Modification tracks what the reviewer changed in a hypothesis.
type Modification struct {
	// HypothesisID identifies which hypothesis was modified.
	HypothesisID string `json:"hypothesis_id"`

	// Field is the JSON path to the modified field.
	Field string `json:"field"`

	// OldValue is the original value (may be any JSON type).
	OldValue any `json:"old_value"`

	// NewValue is the updated value (may be any JSON type).
	NewValue any `json:"new_value"`

	// Reason explains why this change was made.
	Reason string `json:"reason"`
}

// MaxConfidence is the maximum allowed confidence score for MVP.
// This prevents overconfidence in hypotheses.
const MaxConfidence = 0.85

// MaxHypotheses is the maximum number of hypotheses per investigation.
const MaxHypotheses = 3

// ValidateHypothesis checks if a hypothesis meets the required schema constraints.
func ValidateHypothesis(h Hypothesis) error {
	if h.ID == "" {
		return &ValidationError{Field: "id", Message: "hypothesis ID is required"}
	}
	if h.Claim == "" {
		return &ValidationError{Field: "claim", Message: "claim is required"}
	}
	if len(h.SupportingEvidence) == 0 {
		return &ValidationError{Field: "supporting_evidence", Message: "at least one piece of supporting evidence is required"}
	}
	if h.Confidence < 0 || h.Confidence > 1 {
		return &ValidationError{Field: "confidence", Message: "confidence must be between 0.0 and 1.0"}
	}
	if h.Confidence > MaxConfidence {
		return &ValidationError{Field: "confidence", Message: "confidence cannot exceed 0.85 for MVP"}
	}
	if len(h.ValidationPlan.FalsificationChecks) == 0 {
		return &ValidationError{Field: "validation_plan.falsification_checks", Message: "at least one falsification check is required"}
	}
	return nil
}

// ValidationError represents a hypothesis validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "hypothesis validation error: " + e.Field + ": " + e.Message
}
