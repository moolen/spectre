package builder

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

func TestSubmitHypotheses_Success(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitHypothesesArgs{
		Hypotheses: []HypothesisArg{
			{
				ID:    "hyp-1",
				Claim: "The ConfigMap change caused the Pod to crash",
				SupportingEvidence: []EvidenceRefArg{
					{
						Type:        "change",
						SourceID:    "change-1",
						Description: "ConfigMap my-config was updated 5 minutes before incident",
						Strength:    "strong",
					},
				},
				Assumptions: []AssumptionArg{
					{
						Description:         "The pod reads from the ConfigMap on startup",
						IsVerified:          false,
						Falsifiable:         true,
						FalsificationMethod: "Check pod spec for ConfigMap volume mount",
					},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks: []ValidationTaskArg{
						{
							Description: "Check if ConfigMap is mounted by the pod",
							Tool:        "resource_explorer",
							Expected:    "ConfigMap should be mounted as volume",
						},
					},
					FalsificationChecks: []ValidationTaskArg{
						{
							Description: "Check if pod was restarting before the ConfigMap change",
							Tool:        "resource_changes",
							Expected:    "No restarts before the ConfigMap change",
						},
					},
				},
				Confidence: 0.75,
			},
		},
	}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s': %s", result.Status, result.Message)
	}

	// Verify state was updated
	if _, ok := ctx.actions.StateDelta[types.StateKeyRawHypotheses]; !ok {
		t.Error("expected raw hypotheses to be written to state")
	}
	if ctx.actions.StateDelta[types.StateKeyPipelineStage] != types.PipelineStageBuilding {
		t.Errorf("expected pipeline stage to be '%s'", types.PipelineStageBuilding)
	}

	// Verify escalate flag is NOT set (only the final agent sets Escalate=true)
	if ctx.actions.Escalate {
		t.Error("expected Escalate to be false for builder agent")
	}

	// Verify the serialized data
	hypothesesJSON := ctx.actions.StateDelta[types.StateKeyRawHypotheses].(string)
	var hypotheses []types.Hypothesis
	if err := json.Unmarshal([]byte(hypothesesJSON), &hypotheses); err != nil {
		t.Fatalf("failed to unmarshal hypotheses: %v", err)
	}

	if len(hypotheses) != 1 {
		t.Errorf("expected 1 hypothesis, got %d", len(hypotheses))
	}
	if hypotheses[0].ID != "hyp-1" {
		t.Errorf("unexpected hypothesis ID: %s", hypotheses[0].ID)
	}
	if hypotheses[0].Confidence != 0.75 {
		t.Errorf("expected confidence 0.75, got %f", hypotheses[0].Confidence)
	}
	if hypotheses[0].Status != types.HypothesisStatusPending {
		t.Errorf("expected status 'pending', got '%s'", hypotheses[0].Status)
	}
}

func TestSubmitHypotheses_ConfidenceCapped(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitHypothesesArgs{
		Hypotheses: []HypothesisArg{
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
					ConfirmationChecks: []ValidationTaskArg{
						{Description: "test", Expected: "test"},
					},
					FalsificationChecks: []ValidationTaskArg{
						{Description: "test", Expected: "test"},
					},
				},
				Confidence: 0.95, // Above max of 0.85
			},
		},
	}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still succeed but with validation warning
	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s'", result.Status)
	}

	// Check that confidence was capped
	hypothesesJSON := ctx.actions.StateDelta[types.StateKeyRawHypotheses].(string)
	var hypotheses []types.Hypothesis
	if err := json.Unmarshal([]byte(hypothesesJSON), &hypotheses); err != nil {
		t.Fatalf("failed to unmarshal hypotheses: %v", err)
	}

	if hypotheses[0].Confidence != types.MaxConfidence {
		t.Errorf("expected confidence to be capped at %f, got %f", types.MaxConfidence, hypotheses[0].Confidence)
	}

	// Check for warning in validation errors
	if len(result.ValidationErrors) == 0 {
		t.Error("expected validation warning about capped confidence")
	}
}

func TestSubmitHypotheses_NoHypotheses(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitHypothesesArgs{
		Hypotheses: []HypothesisArg{},
	}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if len(result.ValidationErrors) == 0 {
		t.Error("expected validation errors")
	}
}

func TestSubmitHypotheses_TooManyHypotheses(t *testing.T) {
	ctx := newMockToolContext()

	// Create more than MaxHypotheses (3)
	hypotheses := make([]HypothesisArg, 5)
	for i := range hypotheses {
		hypotheses[i] = HypothesisArg{
			ID:    "hyp-" + string(rune('1'+i)),
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
		}
	}

	args := SubmitHypothesesArgs{Hypotheses: hypotheses}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
}

func TestSubmitHypotheses_MissingEvidence(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitHypothesesArgs{
		Hypotheses: []HypothesisArg{
			{
				ID:                 "hyp-1",
				Claim:              "Test hypothesis",
				SupportingEvidence: []EvidenceRefArg{}, // Empty evidence
				Assumptions: []AssumptionArg{
					{Description: "test", Falsifiable: true, FalsificationMethod: "test"},
				},
				ValidationPlan: ValidationPlanArg{
					ConfirmationChecks:  []ValidationTaskArg{{Description: "test", Expected: "test"}},
					FalsificationChecks: []ValidationTaskArg{{Description: "test", Expected: "test"}},
				},
				Confidence: 0.5,
			},
		},
	}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have validation errors
	if len(result.ValidationErrors) == 0 {
		t.Error("expected validation errors for missing evidence")
	}
}

func TestSubmitHypotheses_MissingFalsificationChecks(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitHypothesesArgs{
		Hypotheses: []HypothesisArg{
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
					FalsificationChecks: []ValidationTaskArg{}, // Empty falsification checks
				},
				Confidence: 0.5,
			},
		},
	}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have validation errors
	if len(result.ValidationErrors) == 0 {
		t.Error("expected validation errors for missing falsification checks")
	}
}

func TestSubmitHypotheses_MultipleHypotheses(t *testing.T) {
	ctx := newMockToolContext()

	args := SubmitHypothesesArgs{
		Hypotheses: []HypothesisArg{
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
				Confidence: 0.8,
			},
			{
				ID:    "hyp-2",
				Claim: "Second hypothesis",
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
			},
		},
	}

	result, err := submitHypotheses(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != statusSuccess {
		t.Errorf("expected status 'success', got '%s'", result.Status)
	}

	if result.Message != "Generated 2 hypotheses" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestNewSubmitHypothesesTool_Creation(t *testing.T) {
	tool, err := NewSubmitHypothesesTool()
	if err != nil {
		t.Fatalf("failed to create tool: %v", err)
	}

	if tool.Name() != "submit_hypotheses" {
		t.Errorf("unexpected tool name: %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty tool description")
	}
}
