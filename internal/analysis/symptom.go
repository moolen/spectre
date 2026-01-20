package analysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

const reasonOOMKilled = "OOMKilled"

// ErrNoChangeEventInRange is returned when no ChangeEvent is found within the
// requested time range, but earlier data exists. This allows the handler to
// return HTTP 200 with a hint about when data is available.
type ErrNoChangeEventInRange struct {
	RequestedTimestamp  int64
	RequestedTime       time.Time
	FirstEventTimestamp int64
	FirstEventTime      time.Time
	DiffSeconds         int64
	SuggestedTimestamp  int64 // Only set if timestamp is too early
	TimestampTooEarly   bool
}

// Error implements the error interface
func (e *ErrNoChangeEventInRange) Error() string {
	if e.TimestampTooEarly {
		return fmt.Sprintf(
			"no ChangeEvent found within ±5 minutes of timestamp %d (%s). "+
				"First event for this resource occurred at %s (%d), which is %d seconds later. "+
				"Try using timestamp: %d",
			e.RequestedTimestamp, e.RequestedTime.Format(time.RFC3339),
			e.FirstEventTime.Format(time.RFC3339), e.FirstEventTimestamp,
			e.DiffSeconds, e.SuggestedTimestamp,
		)
	}
	return fmt.Sprintf(
		"no ChangeEvent found within ±5 minutes of timestamp %d (%s). "+
			"First event for this resource occurred at %s (%d), which is %d seconds earlier",
		e.RequestedTimestamp, e.RequestedTime.Format(time.RFC3339),
		e.FirstEventTime.Format(time.RFC3339), e.FirstEventTimestamp,
		e.DiffSeconds,
	)
}

// Hint returns a human-readable hint message for the API response
func (e *ErrNoChangeEventInRange) Hint() string {
	if e.TimestampTooEarly {
		return fmt.Sprintf(
			"No data found at requested time. First event for this resource occurred at %s. "+
				"Try using end=%d for analysis.",
			e.FirstEventTime.Format(time.RFC3339), e.SuggestedTimestamp/1_000_000_000,
		)
	}
	return fmt.Sprintf(
		"No data found at requested time. First event for this resource occurred at %s, "+
			"which is %d seconds earlier than the requested time.",
		e.FirstEventTime.Format(time.RFC3339), e.DiffSeconds,
	)
}

// extractObservedSymptom extracts facts from the failure event (no inference)
func (a *RootCauseAnalyzer) extractObservedSymptom(
	ctx context.Context,
	resourceUID string,
	failureTimestamp int64,
) (*ObservedSymptom, error) {
	// Query for the ChangeEvent at the failure timestamp
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp <= $failureTimestamp + $tolerance
			  AND e.timestamp >= $failureTimestamp - $tolerance
			RETURN e
			ORDER BY abs(e.timestamp - $failureTimestamp) ASC
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"resourceUID":      resourceUID,
			"failureTimestamp": failureTimestamp,
			"tolerance":        int64(300_000_000_000), // 5 minutes
		},
	}

	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query failure event: %w", err)
	}

	if len(result.Rows) == 0 {
		// No event found in the tolerance window - provide helpful diagnostics
		// Query for any events for this resource
		diagnosticQuery := graph.GraphQuery{
			Query: `
				MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
				RETURN e.timestamp
				ORDER BY e.timestamp ASC
				LIMIT 5
			`,
			Parameters: map[string]interface{}{
				"resourceUID": resourceUID,
			},
		}

		diagnosticResult, diagErr := a.graphClient.ExecuteQuery(ctx, diagnosticQuery)
		if diagErr == nil && len(diagnosticResult.Rows) > 0 {
			// Get first event timestamp
			if firstEventTS, ok := diagnosticResult.Rows[0][0].(int64); ok {
				firstEventTime := time.Unix(0, firstEventTS)
				providedTime := time.Unix(0, failureTimestamp)
				diffSeconds := (firstEventTS - failureTimestamp) / 1_000_000_000

				if diffSeconds > 300 {
					// Timestamp is too early - return structured error with hint
					return nil, &ErrNoChangeEventInRange{
						RequestedTimestamp:  failureTimestamp,
						RequestedTime:       providedTime,
						FirstEventTimestamp: firstEventTS,
						FirstEventTime:      firstEventTime,
						DiffSeconds:         diffSeconds,
						SuggestedTimestamp:  firstEventTS,
						TimestampTooEarly:   true,
					}
				} else if diffSeconds < -300 {
					// Timestamp is too late - return structured error with hint
					return nil, &ErrNoChangeEventInRange{
						RequestedTimestamp:  failureTimestamp,
						RequestedTime:       providedTime,
						FirstEventTimestamp: firstEventTS,
						FirstEventTime:      firstEventTime,
						DiffSeconds:         -diffSeconds,
						TimestampTooEarly:   false,
					}
				}
			}
		}

		return nil, fmt.Errorf("no ChangeEvent found for resource %s at timestamp %d", resourceUID, failureTimestamp)
	}

	// Parse the event node
	eventProps, err := graph.ParseNodeFromResult(result.Rows[0][0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse event node: %w", err)
	}
	event := graph.ParseChangeEventFromNode(eventProps)

	// Get resource identity
	resourceQuery := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})
			RETURN r
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
		},
	}

	resourceResult, err := a.graphClient.ExecuteQuery(ctx, resourceQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource: %w", err)
	}

	if len(resourceResult.Rows) == 0 {
		return nil, fmt.Errorf("resource %s not found", resourceUID)
	}

	resourceProps, err := graph.ParseNodeFromResult(resourceResult.Rows[0][0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource node: %w", err)
	}
	resource := graph.ParseResourceIdentityFromNode(resourceProps)

	// Classify symptom type based on error message and container issues
	symptomType := classifySymptomType(event.Status, event.ErrorMessage, event.ContainerIssues)

	return &ObservedSymptom{
		Resource: SymptomResource{
			UID:       resource.UID,
			Kind:      resource.Kind,
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
		Status:       event.Status,
		ErrorMessage: event.ErrorMessage,
		ObservedAt:   time.Unix(0, event.Timestamp),
		SymptomType:  symptomType,
	}, nil
}

// classifySymptomType determines the symptom category from observed facts
func classifySymptomType(status, errorMessage string, containerIssues []string) string {
	// Check container issues first (most specific)
	for _, issue := range containerIssues {
		switch issue {
		case "ImagePullBackOff", "ErrImagePull":
			return "ImagePullError"
		case "CrashLoopBackOff":
			return "CrashLoop"
		case reasonOOMKilled:
			return reasonOOMKilled
		case "ContainerCreating":
			return "ContainerStartup"
		}
	}

	// Check error message patterns (case-insensitive)
	errorLower := strings.ToLower(errorMessage)
	if strings.Contains(errorLower, "image") && (strings.Contains(errorLower, "pull") || strings.Contains(errorLower, "failed")) {
		return "ImagePullError"
	}
	if strings.Contains(errorLower, "crash") || strings.Contains(errorLower, "backoff") {
		return "CrashLoop"
	}
	if strings.Contains(errorLower, "oom") || strings.Contains(errorLower, "out of memory") {
		return reasonOOMKilled
	}
	if strings.Contains(errorLower, "evicted") {
		return "Evicted"
	}
	if strings.Contains(errorLower, "unschedulable") || strings.Contains(errorLower, "insufficient") {
		return "SchedulingFailure"
	}

	// Fallback to status
	switch status {
	case "Error":
		return "Error"
	case "Warning":
		return "Warning"
	case "Terminating":
		return "Terminating"
	case "Pending":
		// Check if it's a scheduling issue
		if strings.Contains(errorLower, "node") || strings.Contains(errorLower, "pending") {
			return "SchedulingFailure"
		}
		return "Pending"
	default:
		return "Unknown"
	}
}
