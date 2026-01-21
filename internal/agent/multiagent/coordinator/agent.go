//go:build disabled

package coordinator

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"

	spectretools "github.com/moolen/spectre/internal/agent/tools"

	"github.com/moolen/spectre/internal/agent/multiagent/rootcause"
)

// AgentName is the name of the Coordinator Agent.
const AgentName = "coordinator_agent"

// AgentDescription is the description of the Coordinator Agent.
const AgentDescription = "Main entry point for Spectre. Routes user requests to appropriate sub-agents for incident investigation."

// New creates a new Coordinator Agent.
//
// The coordinator is the top-level agent that:
// 1. Receives user messages
// 2. Routes incident reports to the root_cause_agent
// 3. Presents results back to the user
//
// Parameters:
//   - llm: The language model adapter (Anthropic via multiagent/model)
//   - registry: The Spectre tools registry for passing to sub-agents
func New(llm model.LLM, registry *spectretools.Registry) (agent.Agent, error) {
	// Create the root cause agent pipeline
	rootCauseAgent, err := rootcause.New(llm, registry)
	if err != nil {
		return nil, err
	}

	// Create the coordinator as an LLM agent with the root cause agent as a sub-agent
	// ADK will automatically create agent transfer tools for sub-agents
	return llmagent.New(llmagent.Config{
		Name:        AgentName,
		Description: AgentDescription,
		Model:       llm,
		Instruction: SystemPrompt,
		SubAgents: []agent.Agent{
			rootCauseAgent,
		},
		// Include conversation history for multi-turn interactions
		IncludeContents: llmagent.IncludeContentsDefault,
	})
}
