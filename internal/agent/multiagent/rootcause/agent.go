//go:build disabled

// Package rootcause implements the RootCauseAgent that orchestrates the incident
// analysis pipeline using ADK's sequential agent pattern.
package rootcause

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/model"

	spectretools "github.com/moolen/spectre/internal/agent/tools"

	"github.com/moolen/spectre/internal/agent/multiagent/builder"
	"github.com/moolen/spectre/internal/agent/multiagent/gathering"
	"github.com/moolen/spectre/internal/agent/multiagent/intake"
	"github.com/moolen/spectre/internal/agent/multiagent/reviewer"
)

// AgentName is the name of the Root Cause Agent.
const AgentName = "root_cause_agent"

// AgentDescription is the description of the Root Cause Agent.
const AgentDescription = "Orchestrates the incident analysis pipeline: intake → gathering → hypothesis building → review"

// New creates a new Root Cause Agent that runs the 4-stage incident analysis pipeline.
//
// The pipeline executes in sequence:
// 1. IncidentIntakeAgent - Extracts structured facts from user's incident description
// 2. GatheringAgent - Collects system data using Spectre tools
// 3. HypothesisBuilderAgent - Generates falsifiable root cause hypotheses
// 4. IncidentReviewerAgent - Quality gate that approves/modifies/rejects hypotheses
//
// Each agent writes its output to shared session state using temp: prefixed keys.
// The pipeline terminates when the reviewer submits reviewed hypotheses.
func New(llm model.LLM, registry *spectretools.Registry) (agent.Agent, error) {
	// Create the intake agent (stage 1)
	intakeAgent, err := intake.New(llm)
	if err != nil {
		return nil, err
	}

	// Create the gathering agent (stage 2)
	gatheringAgent, err := gathering.New(llm, registry)
	if err != nil {
		return nil, err
	}

	// Create the hypothesis builder agent (stage 3)
	builderAgent, err := builder.New(llm)
	if err != nil {
		return nil, err
	}

	// Create the reviewer agent (stage 4)
	reviewerAgent, err := reviewer.New(llm)
	if err != nil {
		return nil, err
	}

	// Create the sequential pipeline
	// Each agent runs in order, passing data via session state
	// The pipeline exits when an agent sets Escalate=true (reviewer does this)
	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        AgentName,
			Description: AgentDescription,
			SubAgents: []agent.Agent{
				intakeAgent,
				gatheringAgent,
				builderAgent,
				reviewerAgent,
			},
		},
	})
}
