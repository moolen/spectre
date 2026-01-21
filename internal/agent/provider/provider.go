//go:build disabled

// Package provider implements LLM provider abstractions for the Spectre agent.
package provider

import (
	"context"
	"encoding/json"
)

// Message represents a conversation message.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`

	// ToolUse is set when the assistant wants to call a tool
	ToolUse []ToolUseBlock `json:"tool_use,omitempty"`

	// ToolResult is set when providing tool execution results (can have multiple for parallel tool calls)
	ToolResult []ToolResultBlock `json:"tool_result,omitempty"`
}

// Role represents the message sender role.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ToolUseBlock represents a tool call request from the model.
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultBlock represents the result of a tool execution.
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ToolDefinition defines a tool that can be called by the model.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// Response represents the model's response.
type Response struct {
	// Content is the text content of the response (may be empty if only tool calls)
	Content string

	// ToolCalls contains any tool use requests from the model
	ToolCalls []ToolUseBlock

	// StopReason indicates why the model stopped generating
	StopReason StopReason

	// Usage contains token usage information
	Usage Usage
}

// StopReason indicates why the model stopped generating.
type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonError     StopReason = "error"
)

// Usage contains token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Provider defines the interface for LLM providers.
type Provider interface {
	// Chat sends messages to the model and returns the complete response.
	// Tools are optional and define what tools the model can call.
	Chat(ctx context.Context, systemPrompt string, messages []Message, tools []ToolDefinition) (*Response, error)

	// Name returns the provider name for logging and display.
	Name() string

	// Model returns the model identifier being used.
	Model() string
}

// Config contains common configuration for providers.
type Config struct {
	// Model is the model identifier (e.g., "claude-sonnet-4-5-20250929")
	Model string

	// MaxTokens is the maximum number of tokens to generate
	MaxTokens int

	// Temperature controls randomness (0.0 = deterministic, 1.0 = creative)
	Temperature float64
}

// DefaultConfig returns sensible defaults for the agent.
func DefaultConfig() Config {
	return Config{
		Model:       "claude-sonnet-4-5-20250929",
		MaxTokens:   4096,
		Temperature: 0.0, // Deterministic for incident response
	}
}

// ContextWindowSizes maps model identifiers to their context window sizes in tokens.
// These are the maximum number of input tokens each model can process.
var ContextWindowSizes = map[string]int{
	// Claude 3.5 models
	"claude-sonnet-4-5-20250929": 200000,
	"claude-3-5-sonnet-20241022": 200000,
	"claude-3-5-sonnet-20240620": 200000,
	"claude-3-5-haiku-20241022":  200000,
	// Claude 3 models
	"claude-3-opus-20240229":   200000,
	"claude-3-sonnet-20240229": 200000,
	"claude-3-haiku-20240307":  200000,
	// Default fallback
	"default": 200000,
}

// GetContextWindowSize returns the context window size for a given model.
// Returns the default size (200k) if the model is not found.
func GetContextWindowSize(model string) int {
	if size, ok := ContextWindowSizes[model]; ok {
		return size
	}
	return ContextWindowSizes["default"]
}
