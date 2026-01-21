//go:build disabled

// Package model provides LLM adapters for the ADK multi-agent system.
package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenario defines a sequence of mock LLM responses loaded from YAML.
type Scenario struct {
	// Name is the scenario identifier.
	Name string `yaml:"name"`

	// Description is a human-readable description of what the scenario tests.
	Description string `yaml:"description,omitempty"`

	// Interactive indicates this scenario waits for external input.
	Interactive bool `yaml:"interactive,omitempty"`

	// Settings contains global timing settings.
	Settings ScenarioSettings `yaml:"settings,omitempty"`

	// ToolResponses defines canned responses for tools (keyed by tool name).
	ToolResponses map[string]MockToolResponse `yaml:"tool_responses,omitempty"`

	// Steps defines the sequence of mock LLM responses.
	Steps []ScenarioStep `yaml:"steps"`
}

// ScenarioSettings contains global timing and behavior settings.
type ScenarioSettings struct {
	// ThinkingDelayMs is the delay in milliseconds before responding (simulates thinking).
	// Default: 2000 (2 seconds)
	ThinkingDelayMs int `yaml:"thinking_delay_ms,omitempty"`

	// ToolDelayMs is the delay in milliseconds per tool call.
	// Default: 500 (0.5 seconds)
	ToolDelayMs int `yaml:"tool_delay_ms,omitempty"`
}

// ScenarioStep defines a single mock LLM response.
type ScenarioStep struct {
	// Trigger is an optional pattern that must be present in the request to activate this step.
	// If empty, the step auto-advances after the previous step completes.
	// Supports simple substring matching or special triggers:
	// - "tool_result:tool_name" - Triggered when tool results for 'tool_name' are received
	// - "user_message" - Triggered on any user message
	// - "contains:text" - Triggered when request contains 'text'
	Trigger string `yaml:"trigger,omitempty"`

	// Text is the text response from the agent.
	Text string `yaml:"text,omitempty"`

	// ToolCalls defines tool calls the mock LLM will make.
	ToolCalls []MockToolCall `yaml:"tool_calls,omitempty"`

	// DelayMs overrides the thinking delay for this step.
	DelayMs int `yaml:"delay_ms,omitempty"`
}

// MockToolCall defines a tool call the mock LLM will make.
type MockToolCall struct {
	// Name is the tool name (e.g., "cluster_health", "ask_user_question").
	Name string `yaml:"name"`

	// Args are the tool arguments.
	Args map[string]interface{} `yaml:"args"`
}

// MockToolResponse defines a canned response for a tool.
type MockToolResponse struct {
	// Success indicates if the tool execution succeeded.
	Success bool `yaml:"success"`

	// Summary is a brief description of what happened.
	Summary string `yaml:"summary,omitempty"`

	// Data is the tool's output data.
	Data interface{} `yaml:"data,omitempty"`

	// Error contains error details if Success is false.
	Error string `yaml:"error,omitempty"`

	// DelayMs is an optional delay before returning the response.
	DelayMs int `yaml:"delay_ms,omitempty"`
}

// DefaultSettings returns sensible defaults for scenario settings.
func DefaultSettings() ScenarioSettings {
	return ScenarioSettings{
		ThinkingDelayMs: 2000, // 2 seconds
		ToolDelayMs:     500,  // 0.5 seconds
	}
}

// LoadScenario loads a scenario from a YAML file.
func LoadScenario(path string) (*Scenario, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// path is user-provided configuration for test/mock scenarios
	// #nosec G304 -- Scenario file path is intentionally configurable for testing
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file %s: %w", path, err)
	}

	var scenario Scenario
	if err := yaml.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("failed to parse scenario YAML: %w", err)
	}

	// Apply default settings
	if scenario.Settings.ThinkingDelayMs == 0 {
		scenario.Settings.ThinkingDelayMs = DefaultSettings().ThinkingDelayMs
	}
	if scenario.Settings.ToolDelayMs == 0 {
		scenario.Settings.ToolDelayMs = DefaultSettings().ToolDelayMs
	}

	if err := scenario.Validate(); err != nil {
		return nil, fmt.Errorf("invalid scenario: %w", err)
	}

	return &scenario, nil
}

// LoadScenarioFromDir loads a scenario by name from the scenarios directory.
// Looks in ~/.spectre/scenarios/<name>.yaml
func LoadScenarioFromDir(name string) (*Scenario, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Try with .yaml extension first, then .yml
	scenariosDir := filepath.Join(home, ".spectre", "scenarios")

	path := filepath.Join(scenariosDir, name+".yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(scenariosDir, name+".yml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("scenario '%s' not found in %s (tried .yaml and .yml)", name, scenariosDir)
		}
	}

	return LoadScenario(path)
}

// Validate checks that the scenario is valid.
func (s *Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("scenario name is required")
	}

	if s.Interactive {
		// Interactive scenarios don't need steps
		return nil
	}

	if len(s.Steps) == 0 {
		return fmt.Errorf("scenario must have at least one step (or be interactive)")
	}

	for i, step := range s.Steps {
		if step.Text == "" && len(step.ToolCalls) == 0 {
			return fmt.Errorf("step[%d]: must have either text or tool_calls", i)
		}

		for j, tc := range step.ToolCalls {
			if tc.Name == "" {
				return fmt.Errorf("step[%d].tool_calls[%d]: name is required", i, j)
			}
		}
	}

	return nil
}

// GetThinkingDelay returns the thinking delay for a step, using step override or default.
func (s *Scenario) GetThinkingDelay(stepIndex int) int {
	if stepIndex < 0 || stepIndex >= len(s.Steps) {
		return s.Settings.ThinkingDelayMs
	}

	step := s.Steps[stepIndex]
	if step.DelayMs > 0 {
		return step.DelayMs
	}
	return s.Settings.ThinkingDelayMs
}

// GetToolDelay returns the tool delay setting.
func (s *Scenario) GetToolDelay() int {
	return s.Settings.ToolDelayMs
}

// GetToolResponse returns the canned response for a tool, or nil if not defined.
func (s *Scenario) GetToolResponse(toolName string) *MockToolResponse {
	if s.ToolResponses == nil {
		return nil
	}
	resp, ok := s.ToolResponses[toolName]
	if !ok {
		return nil
	}
	return &resp
}

// StepMatcher helps determine which step to execute based on request content.
type StepMatcher struct {
	scenario  *Scenario
	stepIndex int
	completed []bool // Track which steps have been completed
}

// NewStepMatcher creates a new step matcher for a scenario.
func NewStepMatcher(scenario *Scenario) *StepMatcher {
	return &StepMatcher{
		scenario:  scenario,
		stepIndex: 0,
		completed: make([]bool, len(scenario.Steps)),
	}
}

// NextStep returns the next step to execute based on the request content.
// Returns nil if no more steps are available.
func (m *StepMatcher) NextStep(requestContent string) *ScenarioStep {
	if m.scenario.Interactive {
		return nil // Interactive mode doesn't use steps
	}

	// Find the next matching step
	for i := m.stepIndex; i < len(m.scenario.Steps); i++ {
		if m.completed[i] {
			continue
		}

		step := &m.scenario.Steps[i]

		// Check if trigger matches (or no trigger = auto-advance)
		if m.matchesTrigger(step.Trigger, requestContent) {
			m.stepIndex = i + 1
			m.completed[i] = true
			return step
		}
	}

	return nil
}

// matchesTrigger checks if the request content matches the trigger pattern.
func (m *StepMatcher) matchesTrigger(trigger, content string) bool {
	if trigger == "" {
		// No trigger = auto-advance
		return true
	}

	// Handle special triggers
	if trigger == "user_message" {
		// Always matches on user message
		return true
	}

	if strings.HasPrefix(trigger, "tool_result:") {
		toolName := strings.TrimPrefix(trigger, "tool_result:")
		// Check if content contains tool result for this tool
		return strings.Contains(content, toolName)
	}

	if strings.HasPrefix(trigger, "contains:") {
		pattern := strings.TrimPrefix(trigger, "contains:")
		return strings.Contains(strings.ToLower(content), strings.ToLower(pattern))
	}

	// Default: simple substring match
	return strings.Contains(strings.ToLower(content), strings.ToLower(trigger))
}

// CurrentStepIndex returns the current step index.
func (m *StepMatcher) CurrentStepIndex() int {
	return m.stepIndex
}

// Reset resets the step matcher to the beginning.
func (m *StepMatcher) Reset() {
	m.stepIndex = 0
	m.completed = make([]bool, len(m.scenario.Steps))
}

// HasMoreSteps returns true if there are more steps to execute.
func (m *StepMatcher) HasMoreSteps() bool {
	return m.stepIndex < len(m.scenario.Steps)
}
