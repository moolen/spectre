//go:build disabled

package tools

import (
	"encoding/json"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
)

// AskUserQuestionArgs defines the input for the ask_user_question tool.
type AskUserQuestionArgs struct {
	// Question is the main question to ask the user.
	Question string `json:"question"`

	// Summary is an optional structured summary to display before the question.
	// Use this to show the user what information you've extracted or understood.
	Summary string `json:"summary,omitempty"`

	// DefaultConfirm indicates if the default action is to confirm (yes).
	// If true, an empty response or "yes"/"y" will be treated as confirmation.
	DefaultConfirm bool `json:"default_confirm,omitempty"`
}

// AskUserQuestionResult is returned after the user responds.
type AskUserQuestionResult struct {
	// Status indicates the result of the tool call.
	// "pending" means waiting for user response.
	Status string `json:"status"`

	// Message provides additional context.
	Message string `json:"message"`
}

// PendingUserQuestion is stored in session state when awaiting user response.
type PendingUserQuestion struct {
	// Question is the question being asked.
	Question string `json:"question"`

	// Summary is the optional summary displayed to the user.
	Summary string `json:"summary,omitempty"`

	// DefaultConfirm indicates the default action.
	DefaultConfirm bool `json:"default_confirm"`

	// AgentName is the name of the agent that asked the question.
	AgentName string `json:"agent_name"`
}

// UserQuestionResponse represents the parsed user response to a question.
type UserQuestionResponse struct {
	// Confirmed is true if the user confirmed (yes/y/empty with default_confirm).
	Confirmed bool `json:"confirmed"`

	// Response is the user's raw response text.
	Response string `json:"response"`

	// HasClarification is true if the user provided additional text beyond yes/no.
	HasClarification bool `json:"has_clarification"`
}

// ParseUserResponse parses a user's response to determine if they confirmed
// or provided clarification.
func ParseUserResponse(response string, defaultConfirm bool) UserQuestionResponse {
	trimmed := strings.TrimSpace(response)
	lower := strings.ToLower(trimmed)

	result := UserQuestionResponse{
		Response: trimmed,
	}

	// Check for explicit yes/no
	switch lower {
	case "yes", "y", "yeah", "yep", "correct", "confirmed", "ok", "okay":
		result.Confirmed = true
		result.HasClarification = false
		return result
	case "no", "n", "nope", "wrong", "incorrect":
		result.Confirmed = false
		result.HasClarification = false
		return result
	case "":
		// Empty response - use default
		result.Confirmed = defaultConfirm
		result.HasClarification = false
		return result
	}

	// Any other response is treated as clarification (not confirmed, needs re-processing)
	result.Confirmed = false
	result.HasClarification = true
	return result
}

// NewAskUserQuestionTool creates the ask_user_question tool.
// This tool allows agents to pause execution and request user input.
func NewAskUserQuestionTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "ask_user_question",
		Description: `Ask the user a question and wait for their response.

Use this tool when you need to:
- Confirm extracted information before proceeding
- Request clarification on ambiguous input
- Get user approval for a proposed action

The tool will display your summary (if provided) and question to the user,
then wait for their response. The user can:
- Confirm with "yes", "y", "ok", etc.
- Reject with "no", "n", etc.
- Provide clarification by typing any other text

After calling this tool, execution will pause until the user responds.
The user's response will be provided to you in the next message.`,
	}, askUserQuestion)
}

// askUserQuestion is the handler for the ask_user_question tool.
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
		AgentName:      ctx.AgentName(),
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
	actions.StateDelta[types.StateKeyPendingUserQuestion] = string(pendingJSON)

	// Escalate to pause execution and return control to the user
	actions.Escalate = true
	actions.SkipSummarization = true

	return AskUserQuestionResult{
		Status:  "pending",
		Message: "Waiting for user response. The user will see your question and can confirm or provide clarification.",
	}, nil
}
