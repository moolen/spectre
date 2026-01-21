//go:build disabled

package intake

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
)

// SubmitIncidentFactsArgs is the input schema for the submit_incident_facts tool.
// The LLM calls this tool with extracted facts from the user's incident description.
type SubmitIncidentFactsArgs struct {
	// Symptoms describes what is failing or broken.
	Symptoms []SymptomArg `json:"symptoms"`

	// IncidentStart is when symptoms first appeared (in user's words).
	IncidentStart string `json:"incident_start,omitempty"`

	// DurationStr is a human-readable duration (e.g., "ongoing for 10 minutes").
	DurationStr string `json:"duration_str,omitempty"`

	// IsOngoing indicates whether the incident is still active.
	IsOngoing bool `json:"is_ongoing"`

	// StartTimestamp is the Unix timestamp (seconds) for the start of the investigation window.
	// This is required and should be calculated by the agent based on user input.
	// If no time is specified by the user, default to now - 15 minutes (900 seconds).
	StartTimestamp int64 `json:"start_timestamp"`

	// EndTimestamp is the Unix timestamp (seconds) for the end of the investigation window.
	// This is required and is typically the current time for ongoing incidents.
	EndTimestamp int64 `json:"end_timestamp"`

	// MitigationsAttempted lists what the user has already tried.
	MitigationsAttempted []MitigationArg `json:"mitigations_attempted,omitempty"`

	// UserConstraints captures any focus areas or exclusions the user specified.
	UserConstraints []string `json:"user_constraints,omitempty"`

	// AffectedResource is set if the user explicitly named a resource.
	AffectedResource *ResourceRefArg `json:"affected_resource,omitempty"`
}

// SymptomArg describes an observed problem (tool input schema).
type SymptomArg struct {
	// Description is the symptom in the user's own words.
	Description string `json:"description"`

	// Resource is the affected resource name if mentioned.
	Resource string `json:"resource,omitempty"`

	// Namespace is the Kubernetes namespace if mentioned.
	Namespace string `json:"namespace,omitempty"`

	// Kind is the Kubernetes resource kind if mentioned (Pod, Deployment, etc.).
	Kind string `json:"kind,omitempty"`

	// Severity is the assessed severity based on user language.
	// Values: critical, high, medium, low
	Severity string `json:"severity"`

	// FirstSeen is when the symptom was first observed (e.g., "10 minutes ago").
	FirstSeen string `json:"first_seen,omitempty"`
}

// MitigationArg describes an attempted remediation (tool input schema).
type MitigationArg struct {
	// Description is what was tried.
	Description string `json:"description"`

	// Result is the outcome if known.
	// Values: "no effect", "partial", "unknown", "made worse"
	Result string `json:"result,omitempty"`
}

// ResourceRefArg identifies a specific Kubernetes resource (tool input schema).
type ResourceRefArg struct {
	// Kind is the resource kind (Pod, Deployment, Service, etc.).
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace"`

	// Name is the resource name.
	Name string `json:"name"`
}

// SubmitIncidentFactsResult is the output of the submit_incident_facts tool.
type SubmitIncidentFactsResult struct {
	// Status indicates whether the submission was successful.
	Status string `json:"status"`

	// Message provides additional information.
	Message string `json:"message"`
}

// NewSubmitIncidentFactsTool creates the submit_incident_facts tool.
// This tool writes the extracted incident facts to session state for the next agent.
func NewSubmitIncidentFactsTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "submit_incident_facts",
		Description: `Submit the extracted incident facts to complete the intake process.

IMPORTANT: Only call this tool AFTER the user has confirmed the extracted information via ask_user_question.

Required fields:
- symptoms: List of observed problems
- start_timestamp: Unix timestamp (seconds) for investigation window start
- end_timestamp: Unix timestamp (seconds) for investigation window end

If the user did not specify a time, default to the last 15 minutes (start = now - 900 seconds, end = now).`,
	}, submitIncidentFacts)
}

// submitIncidentFacts is the handler for the submit_incident_facts tool.
func submitIncidentFacts(ctx tool.Context, args SubmitIncidentFactsArgs) (SubmitIncidentFactsResult, error) {
	now := time.Now()
	nowUnix := now.Unix()

	// Validate and fix timestamps if they're obviously wrong
	// If timestamps are more than 1 year old or in the future, use sensible defaults
	startTs := args.StartTimestamp
	endTs := args.EndTimestamp

	oneYearAgo := nowUnix - (365 * 24 * 3600)
	oneHourFromNow := nowUnix + 3600

	// Check if start timestamp is unreasonable
	if startTs < oneYearAgo || startTs > oneHourFromNow {
		// Default to 15 minutes ago
		startTs = nowUnix - 900
	}

	// Check if end timestamp is unreasonable
	if endTs < oneYearAgo || endTs > oneHourFromNow {
		// Default to now
		endTs = nowUnix
	}

	// Ensure start is before end
	if startTs > endTs {
		startTs, endTs = endTs, startTs
	}

	// Convert tool args to IncidentFacts
	facts := types.IncidentFacts{
		IsOngoing:       args.IsOngoing,
		UserConstraints: args.UserConstraints,
		ExtractedAt:     now,
		Timeline: types.Timeline{
			IncidentStart:  args.IncidentStart,
			DurationStr:    args.DurationStr,
			UserReportedAt: now,
			StartTimestamp: startTs,
			EndTimestamp:   endTs,
		},
	}

	// Convert symptoms
	for _, s := range args.Symptoms {
		facts.Symptoms = append(facts.Symptoms, types.Symptom{
			Description: s.Description,
			Resource:    s.Resource,
			Namespace:   s.Namespace,
			Kind:        s.Kind,
			Severity:    s.Severity,
			FirstSeen:   s.FirstSeen,
		})
	}

	// Convert mitigations
	for _, m := range args.MitigationsAttempted {
		facts.MitigationsAttempted = append(facts.MitigationsAttempted, types.Mitigation{
			Description: m.Description,
			Result:      m.Result,
		})
	}

	// Convert affected resource
	if args.AffectedResource != nil {
		facts.AffectedResource = &types.ResourceRef{
			Kind:      args.AffectedResource.Kind,
			Namespace: args.AffectedResource.Namespace,
			Name:      args.AffectedResource.Name,
		}
	}

	// Serialize to JSON
	factsJSON, err := json.Marshal(facts)
	if err != nil {
		return SubmitIncidentFactsResult{
			Status:  "error",
			Message: fmt.Sprintf("failed to serialize incident facts: %v", err),
		}, err
	}

	// Write to session state for the next agent
	actions := ctx.Actions()
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[types.StateKeyIncidentFacts] = string(factsJSON)
	actions.StateDelta[types.StateKeyPipelineStage] = types.PipelineStageIntake

	// Don't escalate - let the SequentialAgent continue to the next stage
	actions.SkipSummarization = true

	return SubmitIncidentFactsResult{
		Status:  "success",
		Message: fmt.Sprintf("Extracted %d symptoms, %d mitigations", len(facts.Symptoms), len(facts.MitigationsAttempted)),
	}, nil
}
