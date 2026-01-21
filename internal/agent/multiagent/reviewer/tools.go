//go:build disabled

package reviewer

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
)

// SubmitReviewedHypothesesArgs is the input schema for the submit_reviewed_hypotheses tool.
type SubmitReviewedHypothesesArgs struct {
	// Hypotheses contains all hypotheses with their review status.
	Hypotheses []ReviewedHypothesisArg `json:"hypotheses"`

	// ReviewNotes is an overall summary of the review process.
	ReviewNotes string `json:"review_notes"`

	// Modifications lists specific changes made to hypotheses.
	Modifications []ModificationArg `json:"modifications,omitempty"`
}

// ReviewedHypothesisArg represents a reviewed hypothesis (tool input schema).
type ReviewedHypothesisArg struct {
	// ID is a unique identifier for this hypothesis.
	ID string `json:"id"`

	// Claim is the root cause claim (potentially modified by review).
	Claim string `json:"claim"`

	// SupportingEvidence links this hypothesis to specific data.
	SupportingEvidence []EvidenceRefArg `json:"supporting_evidence"`

	// Assumptions lists all assumptions underlying this hypothesis.
	Assumptions []AssumptionArg `json:"assumptions"`

	// ValidationPlan defines how to confirm or falsify this hypothesis.
	ValidationPlan ValidationPlanArg `json:"validation_plan"`

	// Confidence is a calibrated probability score from 0.0 to 0.85.
	Confidence float64 `json:"confidence"`

	// Status indicates the review decision.
	// Values: "approved", "modified", "rejected"
	Status string `json:"status"`

	// RejectionReason is set when Status is "rejected".
	RejectionReason string `json:"rejection_reason,omitempty"`
}

// EvidenceRefArg links a hypothesis to supporting data (tool input schema).
type EvidenceRefArg struct {
	Type        string `json:"type"`
	SourceID    string `json:"source_id"`
	Description string `json:"description"`
	Strength    string `json:"strength"`
}

// AssumptionArg represents an assumption (tool input schema).
type AssumptionArg struct {
	Description         string `json:"description"`
	IsVerified          bool   `json:"is_verified"`
	Falsifiable         bool   `json:"falsifiable"`
	FalsificationMethod string `json:"falsification_method,omitempty"`
}

// ValidationPlanArg defines how to validate a hypothesis (tool input schema).
type ValidationPlanArg struct {
	ConfirmationChecks   []ValidationTaskArg `json:"confirmation_checks"`
	FalsificationChecks  []ValidationTaskArg `json:"falsification_checks"`
	AdditionalDataNeeded []string            `json:"additional_data_needed,omitempty"`
}

// ValidationTaskArg describes a validation check (tool input schema).
type ValidationTaskArg struct {
	Description string `json:"description"`
	Tool        string `json:"tool,omitempty"`
	Command     string `json:"command,omitempty"`
	Expected    string `json:"expected"`
}

// ModificationArg tracks what the reviewer changed in a hypothesis.
type ModificationArg struct {
	// HypothesisID identifies which hypothesis was modified.
	HypothesisID string `json:"hypothesis_id"`

	// Field is the JSON path to the modified field.
	Field string `json:"field"`

	// OldValue is the original value.
	OldValue any `json:"old_value"`

	// NewValue is the updated value.
	NewValue any `json:"new_value"`

	// Reason explains why this change was made.
	Reason string `json:"reason"`
}

// SubmitReviewedHypothesesResult is the output of the submit_reviewed_hypotheses tool.
type SubmitReviewedHypothesesResult struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Approved int    `json:"approved"`
	Modified int    `json:"modified"`
	Rejected int    `json:"rejected"`
}

// NewSubmitReviewedHypothesesTool creates the submit_reviewed_hypotheses tool.
func NewSubmitReviewedHypothesesTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "submit_reviewed_hypotheses",
		Description: `Submit the reviewed hypotheses to complete the review process.
Call this tool exactly once with all hypotheses and their review status (approved/modified/rejected).
Include review notes explaining your overall assessment and any modifications made.`,
	}, submitReviewedHypotheses)
}

// submitReviewedHypotheses is the handler for the submit_reviewed_hypotheses tool.
func submitReviewedHypotheses(ctx tool.Context, args SubmitReviewedHypothesesArgs) (SubmitReviewedHypothesesResult, error) {
	if len(args.Hypotheses) == 0 {
		return SubmitReviewedHypothesesResult{
			Status:  "error",
			Message: "no hypotheses provided for review",
		}, nil
	}

	// Convert and count by status
	hypotheses := make([]types.Hypothesis, 0, len(args.Hypotheses))
	var approved, modified, rejected int

	for _, h := range args.Hypotheses {
		hypothesis := types.Hypothesis{
			ID:              h.ID,
			Claim:           h.Claim,
			Confidence:      h.Confidence,
			RejectionReason: h.RejectionReason,
			CreatedAt:       time.Now(), // Keep original or set new?
		}

		// Map status
		switch h.Status {
		case "approved":
			hypothesis.Status = types.HypothesisStatusApproved
			approved++
		case "modified":
			hypothesis.Status = types.HypothesisStatusModified
			modified++
		case "rejected":
			hypothesis.Status = types.HypothesisStatusRejected
			rejected++
		default:
			hypothesis.Status = types.HypothesisStatusPending
		}

		// Cap confidence at max
		if hypothesis.Confidence > types.MaxConfidence {
			hypothesis.Confidence = types.MaxConfidence
		}

		// Convert supporting evidence
		for _, e := range h.SupportingEvidence {
			hypothesis.SupportingEvidence = append(hypothesis.SupportingEvidence, types.EvidenceRef{
				Type:        types.EvidenceType(e.Type),
				SourceID:    e.SourceID,
				Description: e.Description,
				Strength:    types.EvidenceStrength(e.Strength),
			})
		}

		// Convert assumptions
		for _, a := range h.Assumptions {
			hypothesis.Assumptions = append(hypothesis.Assumptions, types.Assumption{
				Description:         a.Description,
				IsVerified:          a.IsVerified,
				Falsifiable:         a.Falsifiable,
				FalsificationMethod: a.FalsificationMethod,
			})
		}

		// Convert validation plan
		hypothesis.ValidationPlan = types.ValidationPlan{
			AdditionalDataNeeded: h.ValidationPlan.AdditionalDataNeeded,
		}
		for _, c := range h.ValidationPlan.ConfirmationChecks {
			hypothesis.ValidationPlan.ConfirmationChecks = append(hypothesis.ValidationPlan.ConfirmationChecks, types.ValidationTask{
				Description: c.Description,
				Tool:        c.Tool,
				Command:     c.Command,
				Expected:    c.Expected,
			})
		}
		for _, c := range h.ValidationPlan.FalsificationChecks {
			hypothesis.ValidationPlan.FalsificationChecks = append(hypothesis.ValidationPlan.FalsificationChecks, types.ValidationTask{
				Description: c.Description,
				Tool:        c.Tool,
				Command:     c.Command,
				Expected:    c.Expected,
			})
		}

		hypotheses = append(hypotheses, hypothesis)
	}

	// Convert modifications
	modifications := make([]types.Modification, 0, len(args.Modifications))
	for _, m := range args.Modifications {
		modifications = append(modifications, types.Modification{
			HypothesisID: m.HypothesisID,
			Field:        m.Field,
			OldValue:     m.OldValue,
			NewValue:     m.NewValue,
			Reason:       m.Reason,
		})
	}

	// Build reviewed hypotheses output
	reviewed := types.ReviewedHypotheses{
		Hypotheses:    hypotheses,
		ReviewNotes:   args.ReviewNotes,
		Modifications: modifications,
	}

	// Serialize to JSON
	reviewedJSON, err := json.Marshal(reviewed)
	if err != nil {
		return SubmitReviewedHypothesesResult{
			Status:  "error",
			Message: fmt.Sprintf("failed to serialize reviewed hypotheses: %v", err),
		}, err
	}

	// Write to session state
	actions := ctx.Actions()
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[types.StateKeyReviewedHypotheses] = string(reviewedJSON)
	actions.StateDelta[types.StateKeyPipelineStage] = types.PipelineStageReviewing

	// Also write to persistent state for later reference
	actions.StateDelta[types.StateKeyFinalHypotheses] = string(reviewedJSON)

	// This is the final stage - escalate to exit the SequentialAgent pipeline
	actions.Escalate = true
	actions.SkipSummarization = true

	return SubmitReviewedHypothesesResult{
		Status:   "success",
		Message:  fmt.Sprintf("Reviewed %d hypotheses: %d approved, %d modified, %d rejected", len(hypotheses), approved, modified, rejected),
		Approved: approved,
		Modified: modified,
		Rejected: rejected,
	}, nil
}
