//go:build disabled

package incident

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	spectretools "github.com/moolen/spectre/internal/agent/tools"
)

// ============================================================================
// Ask User Question Tool (for Phase 1 confirmation)
// ============================================================================

// AskUserQuestionArgs defines the input for the ask_user_question tool.
type AskUserQuestionArgs struct {
	// Question is the main question to ask the user.
	Question string `json:"question"`

	// Summary is an optional structured summary to display before the question.
	Summary string `json:"summary,omitempty"`

	// DefaultConfirm indicates if the default action is to confirm (yes).
	DefaultConfirm bool `json:"default_confirm,omitempty"`
}

// AskUserQuestionResult is returned after calling the tool.
type AskUserQuestionResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// PendingUserQuestion is stored in session state when awaiting user response.
type PendingUserQuestion struct {
	Question       string `json:"question"`
	Summary        string `json:"summary,omitempty"`
	DefaultConfirm bool   `json:"default_confirm"`
}

// StateKeyPendingUserQuestion is the session state key for pending questions.
const StateKeyPendingUserQuestion = "temp:pending_user_question"

// NewAskUserQuestionTool creates the ask_user_question tool.
func NewAskUserQuestionTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "ask_user_question",
		Description: `Ask the user a question and wait for their response.

Use this tool in Phase 1 to confirm extracted incident information before proceeding.

The tool will display your summary (if provided) and question to the user.
The user can confirm with "yes"/"y", reject with "no"/"n", or provide clarification.

After calling this tool, wait for the user's response in the next message.`,
	}, askUserQuestion)
}

func askUserQuestion(ctx tool.Context, args AskUserQuestionArgs) (AskUserQuestionResult, error) {
	if args.Question == "" {
		return AskUserQuestionResult{
			Status:  "error",
			Message: "question is required",
		}, nil
	}

	// Create the pending question
	pending := PendingUserQuestion{
		Question:       args.Question,
		Summary:        args.Summary,
		DefaultConfirm: args.DefaultConfirm,
	}

	// Serialize to JSON
	pendingJSON, err := json.Marshal(pending)
	if err != nil {
		return AskUserQuestionResult{
			Status:  "error",
			Message: "failed to serialize question",
		}, err
	}

	// Store in session state
	actions := ctx.Actions()
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[StateKeyPendingUserQuestion] = string(pendingJSON)

	// Escalate to pause execution and return control to the user
	actions.Escalate = true
	actions.SkipSummarization = true

	return AskUserQuestionResult{
		Status:  "pending",
		Message: "Waiting for user response. The user will see your question and can confirm or provide clarification.",
	}, nil
}

// ============================================================================
// Complete Analysis Tool (for Phase 4 final output)
// ============================================================================

// CompleteAnalysisArgs defines the input for the complete_analysis tool.
type CompleteAnalysisArgs struct {
	// Hypotheses is the list of reviewed hypotheses.
	Hypotheses []HypothesisArg `json:"hypotheses"`

	// Summary is a brief summary of the investigation.
	Summary string `json:"summary"`

	// ToolCallCount is how many data gathering tool calls were made.
	ToolCallCount int `json:"tool_call_count,omitempty"`
}

// HypothesisArg represents a single hypothesis in the tool input.
type HypothesisArg struct {
	ID          string        `json:"id"`
	Claim       string        `json:"claim"`
	Confidence  float64       `json:"confidence"`
	Evidence    []EvidenceArg `json:"evidence"`
	Assumptions []string      `json:"assumptions"`
	Validation  ValidationArg `json:"validation"`
	Status      string        `json:"status,omitempty"` // approved, modified, rejected
	Rejection   string        `json:"rejection_reason,omitempty"`
}

// EvidenceArg represents a piece of evidence.
type EvidenceArg struct {
	Source  string `json:"source"`  // Tool name that provided this
	Finding string `json:"finding"` // What was found
}

// ValidationArg represents the validation plan.
type ValidationArg struct {
	ToConfirm  []string `json:"to_confirm"`
	ToDisprove []string `json:"to_disprove"`
}

// CompleteAnalysisResult is returned after calling the tool.
type CompleteAnalysisResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// AnalysisOutput is stored in session state with the final results.
type AnalysisOutput struct {
	Hypotheses    []HypothesisArg `json:"hypotheses"`
	Summary       string          `json:"summary"`
	ToolCallCount int             `json:"tool_call_count"`
}

// StateKeyAnalysisOutput is the session state key for final analysis output.
const StateKeyAnalysisOutput = "analysis_output"

// NewCompleteAnalysisTool creates the complete_analysis tool.
func NewCompleteAnalysisTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "complete_analysis",
		Description: `Complete the incident analysis and submit final hypotheses.

Use this tool in Phase 4 after you have:
1. Gathered comprehensive data (5-10+ tool calls)
2. Built 1-3 falsifiable hypotheses
3. Reviewed each hypothesis for quality

Required fields:
- hypotheses: List of reviewed hypotheses with evidence
- summary: Brief summary of findings

Each hypothesis must include:
- id: Unique identifier (e.g., "H1")
- claim: Specific, falsifiable root cause statement
- confidence: 0.0 to 0.85 (never higher)
- evidence: List of findings from data gathering
- assumptions: What must be true
- validation: How to confirm AND disprove`,
	}, completeAnalysis)
}

func completeAnalysis(ctx tool.Context, args CompleteAnalysisArgs) (CompleteAnalysisResult, error) {
	// Validate hypotheses
	if len(args.Hypotheses) == 0 {
		return CompleteAnalysisResult{
			Status:  "error",
			Message: "at least one hypothesis is required",
		}, nil
	}

	if len(args.Hypotheses) > 3 {
		return CompleteAnalysisResult{
			Status:  "error",
			Message: "maximum 3 hypotheses allowed",
		}, nil
	}

	// Validate each hypothesis
	for i, h := range args.Hypotheses {
		if h.Claim == "" {
			return CompleteAnalysisResult{
				Status:  "error",
				Message: "hypothesis " + h.ID + " missing claim",
			}, nil
		}
		if h.Confidence > 0.85 {
			// Cap confidence at 0.85
			args.Hypotheses[i].Confidence = 0.85
		}
		if len(h.Evidence) == 0 {
			return CompleteAnalysisResult{
				Status:  "error",
				Message: "hypothesis " + h.ID + " missing evidence",
			}, nil
		}
		if len(h.Validation.ToDisprove) == 0 {
			return CompleteAnalysisResult{
				Status:  "error",
				Message: "hypothesis " + h.ID + " missing falsification check",
			}, nil
		}
	}

	// Create output
	output := AnalysisOutput{
		Hypotheses:    args.Hypotheses,
		Summary:       args.Summary,
		ToolCallCount: args.ToolCallCount,
	}

	// Serialize to JSON
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return CompleteAnalysisResult{
			Status:  "error",
			Message: "failed to serialize output",
		}, err
	}

	// Store in session state
	actions := ctx.Actions()
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[StateKeyAnalysisOutput] = string(outputJSON)

	// Escalate to complete the pipeline
	actions.Escalate = true

	return CompleteAnalysisResult{
		Status:  "success",
		Message: "Analysis complete. Results have been recorded.",
	}, nil
}

// ============================================================================
// Registry Tool Wrapper (wraps Spectre tools for ADK)
// ============================================================================

// SpectreToolWrapper wraps an existing Spectre tool as an ADK tool.
type SpectreToolWrapper struct {
	spectreTool spectretools.Tool
}

// WrapRegistryTool creates an ADK tool from an existing Spectre tool.
func WrapRegistryTool(t spectretools.Tool) (tool.Tool, error) {
	wrapper := &SpectreToolWrapper{spectreTool: t}
	return functiontool.New(functiontool.Config{
		Name:        t.Name(),
		Description: t.Description(),
	}, wrapper.execute)
}

// execute is the handler that bridges Spectre tools to ADK.
func (w *SpectreToolWrapper) execute(ctx tool.Context, args map[string]any) (map[string]any, error) {
	// Convert args to json.RawMessage for Spectre tools
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to marshal args: %v", err)}, nil
	}

	// Execute the Spectre tool
	result, err := w.spectreTool.Execute(context.Background(), argsJSON)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("tool execution failed: %v", err)}, nil
	}

	// Convert result to map for ADK
	if !result.Success {
		return map[string]any{
			"success": false,
			"error":   result.Error,
		}, nil
	}

	// Serialize and deserialize to convert to map[string]any
	dataJSON, err := json.Marshal(result.Data)
	if err != nil {
		return map[string]any{
			"success": true,
			"summary": result.Summary,
			"data":    fmt.Sprintf("%v", result.Data),
		}, nil
	}

	var dataMap map[string]any
	if err := json.Unmarshal(dataJSON, &dataMap); err != nil {
		return map[string]any{
			"success": true,
			"summary": result.Summary,
			"data":    string(dataJSON),
		}, nil
	}

	return map[string]any{
		"success": true,
		"summary": result.Summary,
		"data":    dataMap,
	}, nil
}
