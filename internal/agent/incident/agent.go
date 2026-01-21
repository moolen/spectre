//go:build disabled

// Package incident implements a single-agent incident response system for Kubernetes clusters.
// The agent operates in phases: intake, gathering, analysis, and review.
package incident

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	spectretools "github.com/moolen/spectre/internal/agent/tools"
)

// AgentName is the name of the Incident Response Agent.
const AgentName = "incident_response_agent"

// AgentDescription describes the agent's purpose.
const AgentDescription = "Investigates Kubernetes incidents through systematic phases: intake, data gathering, hypothesis building, and review."

// New creates a new Incident Response Agent.
//
// The agent operates in four phases:
//  1. INTAKE: Extract facts from user's incident description, confirm with user
//  2. GATHERING: Collect system data using Spectre tools
//  3. ANALYSIS: Build falsifiable hypotheses from gathered data
//  4. REVIEW: Validate hypotheses before presenting to user
//
// Parameters:
//   - llm: The language model adapter
//   - registry: The Spectre tools registry for data gathering
func New(llm model.LLM, registry *spectretools.Registry) (agent.Agent, error) {
	// Build the list of tools
	tools := []tool.Tool{}

	// Add phase management tools
	askUserTool, err := NewAskUserQuestionTool()
	if err != nil {
		return nil, err
	}
	tools = append(tools, askUserTool)

	completeAnalysisTool, err := NewCompleteAnalysisTool()
	if err != nil {
		return nil, err
	}
	tools = append(tools, completeAnalysisTool)

	// Add all Spectre tools from the registry for data gathering
	for _, t := range registry.List() {
		wrapped, err := WrapRegistryTool(t)
		if err != nil {
			return nil, err
		}
		tools = append(tools, wrapped)
	}

	// Get system prompt with current timestamp
	systemPrompt := GetSystemPrompt()

	return llmagent.New(llmagent.Config{
		Name:            AgentName,
		Description:     AgentDescription,
		Model:           llm,
		Instruction:     systemPrompt,
		Tools:           tools,
		IncludeContents: llmagent.IncludeContentsDefault,
	})
}
