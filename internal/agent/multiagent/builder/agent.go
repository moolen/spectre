package builder

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// AgentName is the name of the Hypothesis Builder Agent.
const AgentName = "hypothesis_builder_agent"

// AgentDescription is the description of the Hypothesis Builder Agent for the coordinator.
const AgentDescription = "Generates root cause hypotheses based on gathered system data. Produces falsifiable claims with supporting evidence and validation plans."

// New creates a new Hypothesis Builder Agent.
// The agent uses the provided LLM to generate hypotheses from incident facts and system snapshot.
func New(llm model.LLM) (agent.Agent, error) {
	// Create the submit_hypotheses tool
	submitTool, err := NewSubmitHypothesesTool()
	if err != nil {
		return nil, err
	}

	return llmagent.New(llmagent.Config{
		Name:        AgentName,
		Description: AgentDescription,
		Model:       llm,
		Instruction: SystemPrompt,
		Tools:       []tool.Tool{submitTool},
		// Include conversation history so the agent can see previous context
		IncludeContents: llmagent.IncludeContentsDefault,
	})
}
