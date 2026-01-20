package gathering

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	spectretools "github.com/moolen/spectre/internal/agent/tools"
)

// AgentName is the name of the Gathering Agent.
const AgentName = "information_gathering_agent"

// AgentDescription is the description of the Gathering Agent for the coordinator.
const AgentDescription = "Gathers comprehensive system data using Spectre tools based on incident facts. Does not analyze - only collects data."

// New creates a new Information Gathering Agent.
// The agent uses the provided LLM and Spectre tools to collect incident data.
func New(llm model.LLM, registry *spectretools.Registry) (agent.Agent, error) {
	// Wrap existing Spectre tools for ADK
	spectreTools := registry.List()
	tools := make([]tool.Tool, 0, len(spectreTools)+1)

	// Wrap each Spectre tool
	for _, spectreTool := range spectreTools {
		adkTool, err := WrapSpectreTool(spectreTool)
		if err != nil {
			return nil, err
		}
		tools = append(tools, adkTool)
	}

	// Add the submit_system_snapshot tool
	submitTool, err := NewSubmitSystemSnapshotTool()
	if err != nil {
		return nil, err
	}
	tools = append(tools, submitTool)

	return llmagent.New(llmagent.Config{
		Name:        AgentName,
		Description: AgentDescription,
		Model:       llm,
		Instruction: SystemPrompt,
		Tools:       tools,
		// Include conversation history so the agent can see previous context
		IncludeContents: llmagent.IncludeContentsDefault,
	})
}
