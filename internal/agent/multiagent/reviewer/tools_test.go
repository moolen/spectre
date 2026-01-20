package reviewer

import (
	"context"
	"encoding/json"
	"iter"
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
)

// mockState implements session.State for testing.
type mockState struct {
	data map[string]any
}

func newMockState() *mockState {
	return &mockState{data: make(map[string]any)}
}

func (m *mockState) Get(key string) (any, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return nil, session.ErrStateKeyNotExist
}

func (m *mockState) Set(key string, value any) error {
	m.data[key] = value
	return nil
}

func (m *mockState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range m.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

// mockToolContext implements tool.Context for testing.
type mockToolContext struct {
	context.Context
	state   *mockState
	actions *session.EventActions
}

func newMockToolContext() *mockToolContext {
	return &mockToolContext{
		Context: context.Background(),
		state:   newMockState(),
		actions: &session.EventActions{
			StateDelta: make(map[string]any),
		},
	}
}

func (m *mockToolContext) FunctionCallID() string         { return "test-function-call-id" }
func (m *mockToolContext) Actions() *session.EventActions { return m.actions }
func (m *mockToolContext) SearchMemory(ctx context.Context, query string) (*memory.SearchResponse, error) {
	return &memory.SearchResponse{}, nil
}
func (m *mockToolContext) Artifacts() agent.Artifacts           { return nil }
func (m *mockToolContext) State() session.State                 { return m.state }
func (m *mockToolContext) UserContent() *genai.Content          { return nil }
func (m *mockToolContext) InvocationID() string                 { return "test-invocation-id" }
func (m *mockToolContext) AgentName() string                    { return "test-agent" }
func (m *mockToolContext) ReadonlyState() session.ReadonlyState { return m.state }
func (m *mockToolContext) UserID() string                       { return "test-user" }
func (m *mockToolContext) AppName() string                      { return "test-app" }
func (m *mockToolContext) SessionID() string                    { return "test-session" }
func (m *mockToolContext) Branch() string                       { return "" }

const statusSuccess = "success"

func TestSubmitReviewedHypotheses_AllApproved(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitReviewedHypothesesArgs{
		Hypotheses: []ReviewedHypothesisArg{
			{
				ID:    "hyp-1",
				Claim: "The ConfigMap change caused the Pod to crash",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "change", SourceID: "change-1", Description: "ConfigMap updated", Strength: "strong"},
				},
				Assumptions: []AssumptionArg{
					{Description: "Pod reads from ConfigMap", IsVerified: false, Falsifiable: true, FalsificationMethod: "Check pod spec"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "Check mount", Expected: "ConfigMap mounted"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "Check prior restarts", Expected: "No restarts before"}},
				},
				Confidence: 0.75,
				Status:     "approved",
			},
		},
		ReviewNotes: "Hypothesis is well-supported by evidence",
	}

	result, err := submitReviewedHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s': %s", result.Status, result.Message)
	}
	if result.Approved != 1 {
		t.Errorf("expected 1 approved, got %d", result.Approved)
	}
	if result.Modified != 0 {
		t.Errorf("expected 0 modified, got %d", result.Modified)
	}
	if result.Rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", result.Rejected)
	}

	// Verify state was updated
	if _, ok := ctx.actions.StateDelta[types.StateKeyReviewedHypotheses]; !ok {
		t.Error("expected reviewed hypotheses to be written to state")
	}
	if _, ok := ctx.actions.StateDelta[types.StateKeyFinalHypotheses]; !ok {
		t.Error("expected final hypotheses to be written to state")
	}
	if ctx.actions.StateDelta[types.StateKeyPipelineStage] != types.PipelineStageReviewing {
		t.Errorf("expected pipeline stage to be '%s'", types.PipelineStageReviewing)
	}

	// Verify escalate flag was set
	if !ctx.actions.Escalate {
		t.Error("expected Escalate to be true")
	}

	// Verify the serialized data
	reviewedJSON := ctx.actions.StateDelta[types.StateKeyReviewedHypotheses].(string)
	var reviewed types.ReviewedHypotheses
	if err := json.Unmarshal([]byte(reviewedJSON), &reviewed); err != nil {
		t.Fatalf("failed to unmarshal reviewed hypotheses: %v", err)
	}

	if len(reviewed.Hypotheses) != 1 {
		t.Errorf("expected 1 hypothesis, got %d", len(reviewed.Hypotheses))
	}
	if reviewed.Hypotheses[0].Status != types.HypothesisStatusApproved {
		t.Errorf("expected status 'approved', got '%s'", reviewed.Hypotheses[0].Status)
	}
	if reviewed.ReviewNotes != "Hypothesis is well-supported by evidence" {
		t.Errorf("unexpected review notes: %s", reviewed.ReviewNotes)
	}
}

func TestSubmitReviewedHypotheses_Mixed(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitReviewedHypothesesArgs{
		Hypotheses: []ReviewedHypothesisArg{
			{
				ID:    "hyp-1",
				Claim: "First hypothesis",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "change", SourceID: "1", Description: "test", Strength: "strong"},
				},
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence: 0.75,
				Status:     "approved",
			},
			{
				ID:    "hyp-2",
				Claim: "Second hypothesis - modified",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "anomaly", SourceID: "2", Description: "test", Strength: "moderate"},
				},
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence: 0.6,
				Status:     "modified",
			},
			{
				ID:    "hyp-3",
				Claim: "Third hypothesis - rejected",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "event", SourceID: "3", Description: "weak", Strength: "weak"},
				},
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence:      0.3,
				Status:          "rejected",
				RejectionReason: "Insufficient evidence to support the claim",
			},
		},
		ReviewNotes: "Mixed results from review",
		Modifications: []ModificationArg{
			{
				HypothesisID: "hyp-2",
				Field:        "confidence",
				OldValue:     0.7,
				NewValue:     0.6,
				Reason:       "Reduced confidence due to weak correlation",
			},
		},
	}

	result, err := submitReviewedHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s'", result.Status)
	}
	if result.Approved != 1 {
		t.Errorf("expected 1 approved, got %d", result.Approved)
	}
	if result.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", result.Modified)
	}
	if result.Rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", result.Rejected)
	}

	// Verify the serialized data
	reviewedJSON := ctx.actions.StateDelta[types.StateKeyReviewedHypotheses].(string)
	var reviewed types.ReviewedHypotheses
	if err := json.Unmarshal([]byte(reviewedJSON), &reviewed); err != nil {
		t.Fatalf("failed to unmarshal reviewed hypotheses: %v", err)
	}

	// Check statuses
	for _, h := range reviewed.Hypotheses {
		switch h.ID {
		case "hyp-1":
			if h.Status != types.HypothesisStatusApproved {
				t.Errorf("hyp-1: expected status 'approved', got '%s'", h.Status)
			}
		case "hyp-2":
			if h.Status != types.HypothesisStatusModified {
				t.Errorf("hyp-2: expected status 'modified', got '%s'", h.Status)
			}
		case "hyp-3":
			if h.Status != types.HypothesisStatusRejected {
				t.Errorf("hyp-3: expected status 'rejected', got '%s'", h.Status)
			}
			if h.RejectionReason != "Insufficient evidence to support the claim" {
				t.Errorf("hyp-3: unexpected rejection reason: %s", h.RejectionReason)
			}
		}
	}

	// Check modifications
	if len(reviewed.Modifications) != 1 {
		t.Errorf("expected 1 modification, got %d", len(reviewed.Modifications))
	}
	if reviewed.Modifications[0].HypothesisID != "hyp-2" {
		t.Errorf("unexpected modification hypothesis ID: %s", reviewed.Modifications[0].HypothesisID)
	}
}

func TestSubmitReviewedHypotheses_AllRejected(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitReviewedHypothesesArgs{
		Hypotheses: []ReviewedHypothesisArg{
			{
				ID:    "hyp-1",
				Claim: "Rejected hypothesis",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "change", SourceID: "1", Description: "test", Strength: "weak"},
				},
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence:      0.2,
				Status:          "rejected",
				RejectionReason: "No supporting evidence found",
			},
		},
		ReviewNotes: "All hypotheses rejected due to lack of evidence",
	}

	result, err := submitReviewedHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s'", result.Status)
	}
	if result.Approved != 0 {
		t.Errorf("expected 0 approved, got %d", result.Approved)
	}
	if result.Rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", result.Rejected)
	}
}

func TestSubmitReviewedHypotheses_NoHypotheses(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitReviewedHypothesesArgs{
		Hypotheses:  []ReviewedHypothesisArg{},
		ReviewNotes: "No hypotheses to review",
	}

	result, err := submitReviewedHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
}

func TestSubmitReviewedHypotheses_ConfidenceCapped(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitReviewedHypothesesArgs{
		Hypotheses: []ReviewedHypothesisArg{
			{
				ID:    "hyp-1",
				Claim: "Test hypothesis",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "change", SourceID: "1", Description: "test", Strength: "strong"},
				},
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence: 0.95, // Above max of 0.85
				Status:     "approved",
			},
		},
		ReviewNotes: "Test",
	}

	result, err := submitReviewedHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s'", result.Status)
	}

	// Check that confidence was capped
	reviewedJSON := ctx.actions.StateDelta[types.StateKeyReviewedHypotheses].(string)
	var reviewed types.ReviewedHypotheses
	if err := json.Unmarshal([]byte(reviewedJSON), &reviewed); err != nil {
		t.Fatalf("failed to unmarshal reviewed hypotheses: %v", err)
	}

	if reviewed.Hypotheses[0].Confidence != types.MaxConfidence {
		t.Errorf("expected confidence to be capped at %f, got %f", types.MaxConfidence, reviewed.Hypotheses[0].Confidence)
	}
}

func TestSubmitReviewedHypotheses_UnknownStatus(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitReviewedHypothesesArgs{
		Hypotheses: []ReviewedHypothesisArg{
			{
				ID:    "hyp-1",
				Claim: "Test hypothesis",
				SupportingEvidence: []EvidenceRefArg{
					{Type: "change", SourceID: "1", Description: "test", Strength: "strong"},
				},
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence: 0.5,
				Status:     "unknown_status", // Invalid status
			},
		},
		ReviewNotes: "Test",
	}

	result, err := submitReviewedHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should succeed but with pending status
	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s'", result.Status)
	}

	// Check that status defaulted to pending
	reviewedJSON := ctx.actions.StateDelta[types.StateKeyReviewedHypotheses].(string)
	var reviewed types.ReviewedHypotheses
	if err := json.Unmarshal([]byte(reviewedJSON), &reviewed); err != nil {
		t.Fatalf("failed to unmarshal reviewed hypotheses: %v", err)
	}

	if reviewed.Hypotheses[0].Status != types.HypothesisStatusPending {
		t.Errorf("expected status to default to 'pending', got '%s'", reviewed.Hypotheses[0].Status)
	}
}

func TestNewSubmitReviewedHypothesesTool_Creation(t *testing.T) {
	tool, err := NewSubmitReviewedHypothesesTool()
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	if tool.Name() != "submit_reviewed_hypotheses" {
		t.Errorf("unexpected tool name: %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty tool description")
	}
}
