package intake

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/moolen/spectre/internal/agent/tools"
)

// AgentName is the name of the Intake Agent.
const AgentName = "incident_intake_agent"

// AgentDescription is the description of the Intake Agent for the coordinator.
const AgentDescription = "Extracts facts from user incident descriptions. Does not speculate or diagnose - only extracts what the user explicitly states."

// New creates a new Intake Agent.
// The agent uses the provided LLM to extract incident facts from user messages.
func New(llm model.LLM) (agent.Agent, error) {
	// Create the submit_incident_facts tool
	submitTool, err := NewSubmitIncidentFactsTool()
	if err != nil {
		return nil, err
	}

	// Create the ask_user_question tool for confirmation flow
	askUserTool, err := tools.NewAskUserQuestionTool()
	if err != nil {
		return nil, err
	}

	// Get the system prompt with current timestamp injected
	systemPrompt := GetSystemPrompt()

	return llmagent.New(llmagent.Config{
		Name:        AgentName,
		Description: AgentDescription,
		Model:       llm,
		Instruction: systemPrompt,
		Tools:       []tool.Tool{askUserTool, submitTool},
		// Include conversation history so the agent can see the user message
		IncludeContents: llmagent.IncludeContentsDefault,
	})
}
