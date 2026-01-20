// Package tui provides a terminal user interface for the Spectre multi-agent system
// using Bubble Tea.
package tui

import "time"

// Status represents the current state of an agent or tool.
type Status int

const (
	StatusPending Status = iota
	StatusActive
	StatusCompleted
	StatusError
)

// AgentActivatedMsg is sent when a new agent becomes active.
type AgentActivatedMsg struct {
	Name string
}

// AgentTextMsg is sent when an agent produces text output.
type AgentTextMsg struct {
	Agent   string
	Content string
	IsFinal bool
}

// ToolStartedMsg is sent when a tool call begins.
type ToolStartedMsg struct {
	Agent    string
	ToolID   string // Unique ID for this tool call (for matching with completion)
	ToolName string
}

// ToolCompletedMsg is sent when a tool call completes.
type ToolCompletedMsg struct {
	Agent    string
	ToolID   string // Unique ID for this tool call (for matching with start)
	ToolName string
	Success  bool
	Duration time.Duration
	Summary  string
}

// ContextUpdateMsg is sent when context usage changes.
type ContextUpdateMsg struct {
	Used int
	Max  int
}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct {
	Error error
}

// InputSubmittedMsg is sent when the user submits input.
type InputSubmittedMsg struct {
	Input string
}

// InitialPromptMsg is sent when the TUI starts with an initial prompt.
// This displays the prompt in the content view and triggers processing.
type InitialPromptMsg struct {
	Prompt string
}

// CompletedMsg is sent when the entire operation completes.
type CompletedMsg struct{}

// HypothesesUpdatedMsg is sent when hypotheses are updated.
type HypothesesUpdatedMsg struct {
	Count int
}

// UserQuestionMsg is sent when an agent needs user input via ask_user_question tool.
type UserQuestionMsg struct {
	// Question is the question being asked
	Question string
	// Summary is optional context to display before the question
	Summary string
	// DefaultConfirm indicates if empty response means "yes"
	DefaultConfirm bool
	// AgentName is the agent that asked the question
	AgentName string
}

// waitForEventMsg wraps an event received from the event channel.
type waitForEventMsg struct {
	event interface{}
}

// CommandExecutedMsg is sent when a command finishes executing.
type CommandExecutedMsg struct {
	Success bool
	Message string
	IsInfo  bool // true for info-only messages (help, stats, etc)
}
