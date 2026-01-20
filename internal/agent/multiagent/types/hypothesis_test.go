package types

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestValidateHypothesis_Valid(t *testing.T) {
	h := Hypothesis{
		ID:    "h1",
		Claim: "The payment-service errors are caused by the ConfigMap update at 10:03",
		SupportingEvidence: []EvidenceRef{
			{
				Type:        EvidenceTypeChange,
				SourceID:    "recent_changes/0",
				Description: "ConfigMap update correlates with error spike",
				Strength:    EvidenceStrengthStrong,
			},
		},
		Assumptions: []Assumption{
			{
				Description:         "ConfigMap changes are applied immediately",
				IsVerified:          true,
				Falsifiable:         true,
				FalsificationMethod: "Check pod restart timestamps",
			},
		},
		ValidationPlan: ValidationPlan{
			ConfirmationChecks: []ValidationTask{
				{
					Description: "Verify ConfigMap content changed",
					Tool:        "investigate",
					Expected:    "DB_CONNECTION_STRING value differs",
				},
			},
			FalsificationChecks: []ValidationTask{
				{
					Description: "Check if errors existed before ConfigMap update",
					Tool:        "resource_changes",
					Expected:    "No errors before 10:03",
				},
			},
		},
		Confidence: 0.75,
		Status:     HypothesisStatusPending,
		CreatedAt:  time.Now(),
	}

	if err := ValidateHypothesis(h); err != nil {
		t.Errorf("ValidateHypothesis() returned unexpected error: %v", err)
	}
}

func TestValidateHypothesis_MissingID(t *testing.T) {
	h := Hypothesis{
		Claim: "Some claim",
		SupportingEvidence: []EvidenceRef{
			{Type: EvidenceTypeChange, SourceID: "x", Description: "d", Strength: EvidenceStrengthStrong},
		},
		ValidationPlan: ValidationPlan{
			FalsificationChecks: []ValidationTask{{Description: "d", Expected: "e"}},
		},
		Confidence: 0.5,
	}

	err := ValidateHypothesis(h)
	if err == nil {
		t.Fatal("ValidateHypothesis() should return error for missing ID")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	if valErr.Field != "id" {
		t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, "id")
	}
}

func TestValidateHypothesis_MissingClaim(t *testing.T) {
	h := Hypothesis{
		ID: "h1",
		SupportingEvidence: []EvidenceRef{
			{Type: EvidenceTypeChange, SourceID: "x", Description: "d", Strength: EvidenceStrengthStrong},
		},
		ValidationPlan: ValidationPlan{
			FalsificationChecks: []ValidationTask{{Description: "d", Expected: "e"}},
		},
		Confidence: 0.5,
	}

	err := ValidateHypothesis(h)
	if err == nil {
		t.Fatal("ValidateHypothesis() should return error for missing claim")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	if valErr.Field != "claim" {
		t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, "claim")
	}
}

func TestValidateHypothesis_MissingEvidence(t *testing.T) {
	h := Hypothesis{
		ID:                 "h1",
		Claim:              "Some claim",
		SupportingEvidence: []EvidenceRef{},
		ValidationPlan: ValidationPlan{
			FalsificationChecks: []ValidationTask{{Description: "d", Expected: "e"}},
		},
		Confidence: 0.5,
	}

	err := ValidateHypothesis(h)
	if err == nil {
		t.Fatal("ValidateHypothesis() should return error for missing evidence")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	if valErr.Field != "supporting_evidence" {
		t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, "supporting_evidence")
	}
}

func TestValidateHypothesis_ConfidenceTooHigh(t *testing.T) {
	h := Hypothesis{
		ID:    "h1",
		Claim: "Some claim",
		SupportingEvidence: []EvidenceRef{
			{Type: EvidenceTypeChange, SourceID: "x", Description: "d", Strength: EvidenceStrengthStrong},
		},
		ValidationPlan: ValidationPlan{
			FalsificationChecks: []ValidationTask{{Description: "d", Expected: "e"}},
		},
		Confidence: 0.95, // Exceeds MaxConfidence of 0.85
	}

	err := ValidateHypothesis(h)
	if err == nil {
		t.Fatal("ValidateHypothesis() should return error for confidence > 0.85")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	if valErr.Field != "confidence" {
		t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, "confidence")
	}
}

func TestValidateHypothesis_ConfidenceNegative(t *testing.T) {
	h := Hypothesis{
		ID:    "h1",
		Claim: "Some claim",
		SupportingEvidence: []EvidenceRef{
			{Type: EvidenceTypeChange, SourceID: "x", Description: "d", Strength: EvidenceStrengthStrong},
		},
		ValidationPlan: ValidationPlan{
			FalsificationChecks: []ValidationTask{{Description: "d", Expected: "e"}},
		},
		Confidence: -0.5,
	}

	err := ValidateHypothesis(h)
	if err == nil {
		t.Fatal("ValidateHypothesis() should return error for negative confidence")
	}
}

func TestValidateHypothesis_MissingFalsificationChecks(t *testing.T) {
	h := Hypothesis{
		ID:    "h1",
		Claim: "Some claim",
		SupportingEvidence: []EvidenceRef{
			{Type: EvidenceTypeChange, SourceID: "x", Description: "d", Strength: EvidenceStrengthStrong},
		},
		ValidationPlan: ValidationPlan{
			ConfirmationChecks:  []ValidationTask{{Description: "d", Expected: "e"}},
			FalsificationChecks: []ValidationTask{}, // Empty!
		},
		Confidence: 0.5,
	}

	err := ValidateHypothesis(h)
	if err == nil {
		t.Fatal("ValidateHypothesis() should return error for missing falsification checks")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	if valErr.Field != "validation_plan.falsification_checks" {
		t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, "validation_plan.falsification_checks")
	}
}

func TestHypothesis_JSONSerialization(t *testing.T) {
	h := Hypothesis{
		ID:    "h1",
		Claim: "Test claim with \"special\" characters",
		SupportingEvidence: []EvidenceRef{
			{
				Type:        EvidenceTypeAnomaly,
				SourceID:    "anomalies/123",
				Description: "Error rate anomaly detected",
				Strength:    EvidenceStrengthModerate,
			},
		},
		Assumptions: []Assumption{
			{
				Description: "Network is stable",
				IsVerified:  false,
				Falsifiable: true,
			},
		},
		ValidationPlan: ValidationPlan{
			FalsificationChecks: []ValidationTask{
				{Description: "Check network", Expected: "No packet loss"},
			},
		},
		Confidence:      0.65,
		Status:          HypothesisStatusApproved,
		RejectionReason: "",
		CreatedAt:       time.Now(),
	}

	// Serialize
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Deserialize
	var loaded Hypothesis
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify
	if loaded.ID != h.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, h.ID)
	}
	if loaded.Claim != h.Claim {
		t.Errorf("Claim = %q, want %q", loaded.Claim, h.Claim)
	}
	if loaded.Confidence != h.Confidence {
		t.Errorf("Confidence = %f, want %f", loaded.Confidence, h.Confidence)
	}
	if loaded.Status != h.Status {
		t.Errorf("Status = %q, want %q", loaded.Status, h.Status)
	}
	if len(loaded.SupportingEvidence) != len(h.SupportingEvidence) {
		t.Errorf("SupportingEvidence len = %d, want %d", len(loaded.SupportingEvidence), len(h.SupportingEvidence))
	}
}

func TestReviewedHypotheses_JSONSerialization(t *testing.T) {
	reviewed := ReviewedHypotheses{
		Hypotheses: []Hypothesis{
			{
				ID:    "h1",
				Claim: "Root cause is ConfigMap",
				SupportingEvidence: []EvidenceRef{
					{Type: EvidenceTypeChange, SourceID: "changes/0", Description: "d", Strength: EvidenceStrengthStrong},
				},
				ValidationPlan: ValidationPlan{
					FalsificationChecks: []ValidationTask{{Description: "d", Expected: "e"}},
				},
				Confidence: 0.70,
				Status:     HypothesisStatusModified,
			},
			{
				ID:              "h2",
				Claim:           "Root cause is network",
				Status:          HypothesisStatusRejected,
				RejectionReason: "No network issues found in cluster health",
			},
		},
		ReviewNotes: "Modified h1 confidence from 0.90 to 0.70. Rejected h2 due to lack of evidence.",
		Modifications: []Modification{
			{
				HypothesisID: "h1",
				Field:        "confidence",
				OldValue:     0.90,
				NewValue:     0.70,
				Reason:       "Evidence strength does not support high confidence",
			},
		},
	}

	// Serialize
	data, err := json.Marshal(reviewed)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Deserialize
	var loaded ReviewedHypotheses
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify
	if len(loaded.Hypotheses) != 2 {
		t.Errorf("Hypotheses len = %d, want 2", len(loaded.Hypotheses))
	}
	if loaded.ReviewNotes != reviewed.ReviewNotes {
		t.Errorf("ReviewNotes = %q, want %q", loaded.ReviewNotes, reviewed.ReviewNotes)
	}
	if len(loaded.Modifications) != 1 {
		t.Errorf("Modifications len = %d, want 1", len(loaded.Modifications))
	}

	// Check rejected hypothesis
	if loaded.Hypotheses[1].Status != HypothesisStatusRejected {
		t.Errorf("Hypotheses[1].Status = %q, want %q", loaded.Hypotheses[1].Status, HypothesisStatusRejected)
	}
	if loaded.Hypotheses[1].RejectionReason == "" {
		t.Error("Hypotheses[1].RejectionReason should not be empty")
	}
}

func TestHypothesisStatus_Values(t *testing.T) {
	// Test that status constants have expected string values
	tests := []struct {
		status   HypothesisStatus
		expected string
	}{
		{HypothesisStatusPending, "pending"},
		{HypothesisStatusApproved, "approved"},
		{HypothesisStatusModified, "modified"},
		{HypothesisStatusRejected, "rejected"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("HypothesisStatus = %q, want %q", tt.status, tt.expected)
		}
	}
}

func TestEvidenceType_Values(t *testing.T) {
	tests := []struct {
		evidenceType EvidenceType
		expected     string
	}{
		{EvidenceTypeCausalPath, "causal_path"},
		{EvidenceTypeAnomaly, "anomaly"},
		{EvidenceTypeChange, "change"},
		{EvidenceTypeEvent, "event"},
		{EvidenceTypeResourceState, "resource_state"},
		{EvidenceTypeClusterHealth, "cluster_health"},
	}

	for _, tt := range tests {
		if string(tt.evidenceType) != tt.expected {
			t.Errorf("EvidenceType = %q, want %q", tt.evidenceType, tt.expected)
		}
	}
}

func TestEvidenceStrength_Values(t *testing.T) {
	tests := []struct {
		strength EvidenceStrength
		expected string
	}{
		{EvidenceStrengthStrong, "strong"},
		{EvidenceStrengthModerate, "moderate"},
		{EvidenceStrengthWeak, "weak"},
	}

	for _, tt := range tests {
		if string(tt.strength) != tt.expected {
			t.Errorf("EvidenceStrength = %q, want %q", tt.strength, tt.expected)
		}
	}
}

func TestMaxConfidence_Value(t *testing.T) {
	if MaxConfidence != 0.85 {
		t.Errorf("MaxConfidence = %f, want 0.85", MaxConfidence)
	}
}

func TestMaxHypotheses_Value(t *testing.T) {
	if MaxHypotheses != 3 {
		t.Errorf("MaxHypotheses = %d, want 3", MaxHypotheses)
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "confidence",
		Message: "must be between 0 and 1",
	}

	expected := "hypothesis validation error: confidence: must be between 0 and 1"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}
