//go:build disabled

package types

// State keys for inter-agent communication via ADK session state.
// Keys with the "temp:" prefix are transient and cleared after each invocation.
// This follows ADK's state scoping conventions.
const (
	// Pipeline input - the original user message that triggered the investigation.
	StateKeyUserMessage = "temp:user_message"

	// Agent outputs - JSON-encoded output from each pipeline stage.
	// These are written by each agent and read by subsequent agents.

	// StateKeyIncidentFacts contains the IncidentFacts JSON from IncidentIntakeAgent.
	StateKeyIncidentFacts = "temp:incident_facts"

	// StateKeySystemSnapshot contains the SystemSnapshot JSON from InformationGatheringAgent.
	StateKeySystemSnapshot = "temp:system_snapshot"

	// StateKeyRawHypotheses contains the []Hypothesis JSON from HypothesisBuilderAgent.
	StateKeyRawHypotheses = "temp:raw_hypotheses"

	// StateKeyReviewedHypotheses contains the ReviewedHypotheses JSON from IncidentReviewerAgent.
	StateKeyReviewedHypotheses = "temp:reviewed_hypotheses"

	// Pipeline metadata - tracks pipeline execution state.

	// StateKeyPipelineStarted is set to "true" when the pipeline begins.
	StateKeyPipelineStarted = "temp:pipeline_started"

	// StateKeyPipelineError contains error details if the pipeline fails.
	StateKeyPipelineError = "temp:pipeline_error"

	// StateKeyPipelineStage tracks which stage is currently executing.
	// Values: "intake", "gathering", "building", "reviewing", "complete"
	StateKeyPipelineStage = "temp:pipeline_stage"

	// Investigation context - preserved across follow-up questions within a session.

	// StateKeyCurrentInvestigation contains the current investigation ID.
	StateKeyCurrentInvestigation = "investigation_id"

	// StateKeyFinalHypotheses contains the final reviewed hypotheses for persistence.
	// This uses a non-temp key so it persists beyond the current invocation.
	StateKeyFinalHypotheses = "final_hypotheses"
)

// Pipeline stage constants for StateKeyPipelineStage.
const (
	PipelineStageIntake    = "intake"
	PipelineStageGathering = "gathering"
	PipelineStageBuilding  = "building"
	PipelineStageReviewing = "reviewing"
	PipelineStageComplete  = "complete"
)

// User interaction state keys.
const (
	// StateKeyPendingUserQuestion contains the question awaiting user response.
	// When set, the runner should pause execution and display the question to the user.
	// Value is JSON-encoded PendingUserQuestion from tools package.
	StateKeyPendingUserQuestion = "temp:pending_user_question"

	// StateKeyUserConfirmationResponse contains the user's response to a confirmation question.
	// Value is JSON-encoded UserQuestionResponse from tools package.
	StateKeyUserConfirmationResponse = "temp:user_confirmation_response"
)
