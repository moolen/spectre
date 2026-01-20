package builder

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
)

// SubmitHypothesesArgs is the input schema for the submit_hypotheses tool.
type SubmitHypothesesArgs struct {
	// Hypotheses contains the generated root cause hypotheses.
	Hypotheses []HypothesisArg `json:"hypotheses"`
}

// HypothesisArg represents a root-cause hypothesis (tool input schema).
type HypothesisArg struct {
	// ID is a unique identifier for this hypothesis within the investigation.
	ID string `json:"id"`

	// Claim is a clear, falsifiable statement of what is believed to be the root cause.
	Claim string `json:"claim"`

	// SupportingEvidence links this hypothesis to specific data from the SystemSnapshot.
	SupportingEvidence []EvidenceRefArg `json:"supporting_evidence"`

	// Assumptions lists all explicit and implicit assumptions underlying this hypothesis.
	Assumptions []AssumptionArg `json:"assumptions"`

	// ValidationPlan defines how to confirm or falsify this hypothesis.
	ValidationPlan ValidationPlanArg `json:"validation_plan"`

	// Confidence is a calibrated probability score from 0.0 to 0.85.
	Confidence float64 `json:"confidence"`
}

// EvidenceRefArg links a hypothesis to supporting data (tool input schema).
type EvidenceRefArg struct {
	// Type categorizes the kind of evidence.
	// Values: "causal_path", "anomaly", "change", "event", "resource_state", "cluster_health"
	Type string `json:"type"`

	// SourceID is a reference to a specific item in the SystemSnapshot.
	SourceID string `json:"source_id"`

	// Description explains what this evidence shows in relation to the claim.
	Description string `json:"description"`

	// Strength indicates how strongly this evidence supports the claim.
	// Values: "strong", "moderate", "weak"
	Strength string `json:"strength"`
}

// AssumptionArg represents an assumption in a hypothesis (tool input schema).
type AssumptionArg struct {
	// Description is a clear statement of the assumption.
	Description string `json:"description"`

	// IsVerified indicates whether this assumption has been verified.
	IsVerified bool `json:"is_verified"`

	// Falsifiable indicates whether this assumption can be disproven.
	Falsifiable bool `json:"falsifiable"`

	// FalsificationMethod describes how to disprove this assumption.
	FalsificationMethod string `json:"falsification_method,omitempty"`
}

// ValidationPlanArg defines how to confirm or falsify a hypothesis (tool input schema).
type ValidationPlanArg struct {
	// ConfirmationChecks are tests that would support the hypothesis if they pass.
	ConfirmationChecks []ValidationTaskArg `json:"confirmation_checks"`

	// FalsificationChecks are tests that would disprove the hypothesis if they pass.
	FalsificationChecks []ValidationTaskArg `json:"falsification_checks"`

	// AdditionalDataNeeded lists information gaps that would help evaluate this hypothesis.
	AdditionalDataNeeded []string `json:"additional_data_needed,omitempty"`
}

// ValidationTaskArg describes a specific check to perform (tool input schema).
type ValidationTaskArg struct {
	// Description is a human-readable explanation of what to check.
	Description string `json:"description"`

	// Tool is the Spectre tool to use for this check (optional).
	Tool string `json:"tool,omitempty"`

	// Command is a kubectl or other CLI command suggestion (optional).
	Command string `json:"command,omitempty"`

	// Expected describes the expected result if the hypothesis is true/false.
	Expected string `json:"expected"`
}

// SubmitHypothesesResult is the output of the submit_hypotheses tool.
type SubmitHypothesesResult struct {
	Status           string   `json:"status"`
	Message          string   `json:"message"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
}

// NewSubmitHypothesesTool creates the submit_hypotheses tool.
func NewSubmitHypothesesTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "submit_hypotheses",
		Description: `Submit the generated root cause hypotheses to complete the hypothesis building phase.
Call this tool exactly once with all the hypotheses you have generated.
Each hypothesis must have at least one piece of supporting evidence and one falsification check.
Maximum 3 hypotheses, maximum confidence 0.85.`,
	}, submitHypotheses)
}

// submitHypotheses is the handler for the submit_hypotheses tool.
func submitHypotheses(ctx tool.Context, args SubmitHypothesesArgs) (SubmitHypothesesResult, error) {
	// Validate hypothesis count
	if len(args.Hypotheses) == 0 {
		return SubmitHypothesesResult{
			Status:           "error",
			Message:          "at least one hypothesis is required",
			ValidationErrors: []string{"no hypotheses provided"},
		}, nil
	}
	if len(args.Hypotheses) > types.MaxHypotheses {
		return SubmitHypothesesResult{
			Status:           "error",
			Message:          fmt.Sprintf("maximum %d hypotheses allowed", types.MaxHypotheses),
			ValidationErrors: []string{fmt.Sprintf("too many hypotheses: %d > %d", len(args.Hypotheses), types.MaxHypotheses)},
		}, nil
	}

	// Convert and validate each hypothesis
	hypotheses := make([]types.Hypothesis, 0, len(args.Hypotheses))
	var validationErrors []string

	for i, h := range args.Hypotheses {
		hypothesis := types.Hypothesis{
			ID:             h.ID,
			Claim:          h.Claim,
			Confidence:     h.Confidence,
			Status:         types.HypothesisStatusPending,
			CreatedAt:      time.Now(),
			ValidationPlan: types.ValidationPlan{},
		}

		// Cap confidence at max
		if hypothesis.Confidence > types.MaxConfidence {
			hypothesis.Confidence = types.MaxConfidence
			validationErrors = append(validationErrors, fmt.Sprintf("hypothesis %s: confidence capped at %.2f", h.ID, types.MaxConfidence))
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
		hypothesis.ValidationPlan.AdditionalDataNeeded = h.ValidationPlan.AdditionalDataNeeded

		// Validate the hypothesis
		if err := types.ValidateHypothesis(hypothesis); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("hypothesis %d (%s): %v", i, h.ID, err))
		}

		hypotheses = append(hypotheses, hypothesis)
	}

	// If there are critical validation errors, return them
	if len(validationErrors) > 0 {
		// Still serialize if we have hypotheses (non-critical errors like capped confidence)
		if len(hypotheses) == 0 {
			return SubmitHypothesesResult{
				Status:           "error",
				Message:          "hypothesis validation failed",
				ValidationErrors: validationErrors,
			}, nil
		}
	}

	// Serialize to JSON
	hypothesesJSON, err := json.Marshal(hypotheses)
	if err != nil {
		return SubmitHypothesesResult{
			Status:  "error",
			Message: fmt.Sprintf("failed to serialize hypotheses: %v", err),
		}, err
	}

	// Write to session state for the next agent
	actions := ctx.Actions()
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[types.StateKeyRawHypotheses] = string(hypothesesJSON)
	actions.StateDelta[types.StateKeyPipelineStage] = types.PipelineStageBuilding

	// Don't escalate - let the SequentialAgent continue to the next stage
	actions.SkipSummarization = true

	result := SubmitHypothesesResult{
		Status:           "success",
		Message:          fmt.Sprintf("Generated %d hypotheses", len(hypotheses)),
		ValidationErrors: validationErrors,
	}

	return result, nil
}
