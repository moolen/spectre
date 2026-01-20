// Package model provides LLM adapters for the ADK multi-agent system.
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// MockLLM implements model.LLM for testing without real API calls.
// It can run pre-scripted scenarios from YAML or accept interactive input.
type MockLLM struct {
	scenario    *Scenario
	matcher     *StepMatcher
	interactive bool

	// Interactive mode
	inputServer *MockInputServer

	// Timing
	thinkingDelay time.Duration
	toolDelay     time.Duration

	// State tracking
	mu              sync.Mutex
	requestCount    int
	conversationLog []ConversationEntry
}

// ConversationEntry records a request/response pair for debugging.
type ConversationEntry struct {
	Timestamp time.Time
	Request   string
	Response  string
	ToolCalls []string
}

// MockLLMOption configures a MockLLM.
type MockLLMOption func(*MockLLM)

// WithThinkingDelay sets the thinking delay.
func WithThinkingDelay(d time.Duration) MockLLMOption {
	return func(m *MockLLM) {
		m.thinkingDelay = d
	}
}

// WithToolDelay sets the per-tool delay.
func WithToolDelay(d time.Duration) MockLLMOption {
	return func(m *MockLLM) {
		m.toolDelay = d
	}
}

// WithInputServer sets the input server for interactive mode.
func WithInputServer(server *MockInputServer) MockLLMOption {
	return func(m *MockLLM) {
		m.inputServer = server
		m.interactive = true
	}
}

// NewMockLLM creates a MockLLM from a scenario file path.
func NewMockLLM(scenarioPath string, opts ...MockLLMOption) (*MockLLM, error) {
	scenario, err := LoadScenario(scenarioPath)
	if err != nil {
		return nil, err
	}
	return NewMockLLMFromScenario(scenario, opts...)
}

// NewMockLLMFromName creates a MockLLM from a scenario name (loaded from ~/.spectre/scenarios/).
func NewMockLLMFromName(name string, opts ...MockLLMOption) (*MockLLM, error) {
	scenario, err := LoadScenarioFromDir(name)
	if err != nil {
		return nil, err
	}
	return NewMockLLMFromScenario(scenario, opts...)
}

// NewMockLLMFromScenario creates a MockLLM from a loaded scenario.
func NewMockLLMFromScenario(scenario *Scenario, opts ...MockLLMOption) (*MockLLM, error) {
	m := &MockLLM{
		scenario:      scenario,
		matcher:       NewStepMatcher(scenario),
		interactive:   scenario.Interactive,
		thinkingDelay: time.Duration(scenario.Settings.ThinkingDelayMs) * time.Millisecond,
		toolDelay:     time.Duration(scenario.Settings.ToolDelayMs) * time.Millisecond,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// NewMockLLMInteractive creates a MockLLM in interactive mode.
func NewMockLLMInteractive(port int, opts ...MockLLMOption) (*MockLLM, error) {
	server, err := NewMockInputServer(port)
	if err != nil {
		return nil, fmt.Errorf("failed to create input server: %w", err)
	}

	// Create a minimal interactive scenario
	scenario := &Scenario{
		Name:        "interactive",
		Description: "Interactive mode - responses from external input",
		Interactive: true,
		Settings:    DefaultSettings(),
	}

	m := &MockLLM{
		scenario:      scenario,
		matcher:       NewStepMatcher(scenario),
		interactive:   true,
		inputServer:   server,
		thinkingDelay: time.Duration(scenario.Settings.ThinkingDelayMs) * time.Millisecond,
		toolDelay:     time.Duration(scenario.Settings.ToolDelayMs) * time.Millisecond,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// Name returns the model identifier.
func (m *MockLLM) Name() string {
	if m.scenario != nil {
		return fmt.Sprintf("mock:%s", m.scenario.Name)
	}
	return "mock"
}

// InputServer returns the input server (for interactive mode).
func (m *MockLLM) InputServer() *MockInputServer {
	return m.inputServer
}

// GenerateContent implements model.LLM.GenerateContent.
func (m *MockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.requestCount++
		requestNum := m.requestCount
		m.mu.Unlock()

		// Extract request content for logging and trigger matching
		requestContent := extractRequestContent(req)

		// Simulate thinking delay
		thinkingDelay := m.thinkingDelay
		if m.scenario != nil && !m.interactive {
			thinkingDelay = time.Duration(m.scenario.GetThinkingDelay(m.matcher.CurrentStepIndex())) * time.Millisecond
		}

		select {
		case <-ctx.Done():
			yield(nil, ctx.Err())
			return
		case <-time.After(thinkingDelay):
		}

		var resp *model.LLMResponse
		var err error

		if m.interactive {
			resp, err = m.generateInteractiveResponse(ctx, requestContent, requestNum)
		} else {
			resp, err = m.generateScriptedResponse(ctx, requestContent, requestNum)
		}

		if err != nil {
			yield(nil, err)
			return
		}

		// Log the conversation
		m.logConversation(requestContent, resp)

		yield(resp, nil)
	}
}

// generateScriptedResponse generates a response from the scenario steps.
func (m *MockLLM) generateScriptedResponse(ctx context.Context, requestContent string, _ int) (*model.LLMResponse, error) {
	step := m.matcher.NextStep(requestContent)
	if step == nil {
		// No more steps - return a generic completion message
		return &model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{
					{Text: "[Mock scenario completed - no more steps]"},
				},
				Role: "model",
			},
			FinishReason: genai.FinishReasonStop,
			TurnComplete: true,
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     100,
				CandidatesTokenCount: 10,
				TotalTokenCount:      110,
			},
		}, nil
	}

	return m.buildResponseFromStep(ctx, step)
}

// generateInteractiveResponse waits for input from the external server.
func (m *MockLLM) generateInteractiveResponse(ctx context.Context, _ string, _ int) (*model.LLMResponse, error) {
	if m.inputServer == nil {
		return nil, fmt.Errorf("interactive mode requires an input server")
	}

	// Wait for input from the external client
	input, err := m.inputServer.WaitForInput(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get interactive input: %w", err)
	}

	// Build response from input
	return m.buildResponseFromInput(input)
}

// buildResponseFromStep converts a scenario step to an LLM response.
func (m *MockLLM) buildResponseFromStep(ctx context.Context, step *ScenarioStep) (*model.LLMResponse, error) {
	parts := make([]*genai.Part, 0, 1+len(step.ToolCalls))

	// Add text content
	if step.Text != "" {
		parts = append(parts, &genai.Part{
			Text: step.Text,
		})
	}

	// Add tool calls with delays
	for i, tc := range step.ToolCalls {
		// Simulate tool delay (except for first tool)
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(m.toolDelay):
			}
		}

		args := tc.Args
		if args == nil {
			args = make(map[string]interface{})
		}

		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   fmt.Sprintf("mock_call_%d", i),
				Name: tc.Name,
				Args: args,
			},
		})
	}

	// Determine finish reason
	finishReason := genai.FinishReasonStop
	if len(step.ToolCalls) > 0 {
		// When there are tool calls, we still use Stop but TurnComplete should be false
		// to indicate we're waiting for tool results
	}

	return &model.LLMResponse{
		Content: &genai.Content{
			Parts: parts,
			Role:  "model",
		},
		FinishReason: finishReason,
		TurnComplete: true,
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			// Mock token counts - values are estimates and always reasonable for int32
			// #nosec G115 -- Mock estimates are bounded and will never overflow int32
			PromptTokenCount:     int32(len(parts) * 50), // Rough estimate
			CandidatesTokenCount: int32(len(step.Text) / 4), // #nosec G115 -- Safe conversion, bounded values
			TotalTokenCount:      int32(len(parts)*50 + len(step.Text)/4), // #nosec G115 -- Safe conversion, bounded values
		},
	}, nil
}

// buildResponseFromInput converts interactive input to an LLM response.
func (m *MockLLM) buildResponseFromInput(input *InteractiveInput) (*model.LLMResponse, error) {
	parts := make([]*genai.Part, 0, 1+len(input.ToolCalls))

	// Add text content
	if input.Text != "" {
		parts = append(parts, &genai.Part{
			Text: input.Text,
		})
	}

	// Add tool calls
	for i, tc := range input.ToolCalls {
		args := tc.Args
		if args == nil {
			args = make(map[string]interface{})
		}

		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   fmt.Sprintf("mock_call_%d", i),
				Name: tc.Name,
				Args: args,
			},
		})
	}

	return &model.LLMResponse{
		Content: &genai.Content{
			Parts: parts,
			Role:  "model",
		},
		FinishReason: genai.FinishReasonStop,
		TurnComplete: true,
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount: 100,
			// Mock token counts - text length divided by 4 is always reasonable for int32
			// #nosec G115 -- Mock estimates are bounded and will never overflow int32
			CandidatesTokenCount: int32(len(input.Text) / 4),
			TotalTokenCount:      int32(100 + len(input.Text)/4), // #nosec G115 -- Safe conversion, bounded values
		},
	}, nil
}

// logConversation records a conversation entry for debugging.
func (m *MockLLM) logConversation(request string, resp *model.LLMResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := ConversationEntry{
		Timestamp: time.Now(),
		Request:   truncateString(request, 200),
	}

	if resp != nil && resp.Content != nil {
		var textParts []string
		var toolCalls []string

		for _, part := range resp.Content.Parts {
			if part.Text != "" {
				textParts = append(textParts, truncateString(part.Text, 100))
			}
			if part.FunctionCall != nil {
				toolCalls = append(toolCalls, part.FunctionCall.Name)
			}
		}

		entry.Response = strings.Join(textParts, " | ")
		entry.ToolCalls = toolCalls
	}

	m.conversationLog = append(m.conversationLog, entry)
}

// GetConversationLog returns the conversation log for debugging.
func (m *MockLLM) GetConversationLog() []ConversationEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ConversationEntry{}, m.conversationLog...)
}

// Reset resets the MockLLM state for a new conversation.
func (m *MockLLM) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.matcher.Reset()
	m.requestCount = 0
	m.conversationLog = nil
}

// extractRequestContent extracts text content from an LLM request for logging and matching.
func extractRequestContent(req *model.LLMRequest) string {
	if req == nil || len(req.Contents) == 0 {
		return ""
	}

	var parts []string
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part == nil {
				continue
			}
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
			if part.FunctionResponse != nil {
				// Include tool results in content for trigger matching
				respJSON, _ := json.Marshal(part.FunctionResponse.Response)
				parts = append(parts, fmt.Sprintf("[tool_result:%s] %s", part.FunctionResponse.Name, string(respJSON)))
			}
		}
	}

	return strings.Join(parts, "\n")
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure MockLLM implements model.LLM at compile time.
var _ model.LLM = (*MockLLM)(nil)
