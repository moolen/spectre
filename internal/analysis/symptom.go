package analysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

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
					// Timestamp is too early
					return nil, fmt.Errorf(
						"no ChangeEvent found within ±5 minutes of timestamp %d (%s). "+
							"First event for this resource occurred at %s (%d), which is %d seconds later. "+
							"Try using timestamp: %d",
						failureTimestamp, providedTime.Format(time.RFC3339),
						firstEventTime.Format(time.RFC3339), firstEventTS,
						diffSeconds, firstEventTS,
					)
				} else if diffSeconds < -300 {
					// Timestamp is too late
					return nil, fmt.Errorf(
						"no ChangeEvent found within ±5 minutes of timestamp %d (%s). "+
							"First event for this resource occurred at %s (%d), which is %d seconds earlier",
						failureTimestamp, providedTime.Format(time.RFC3339),
						firstEventTime.Format(time.RFC3339), firstEventTS,
						-diffSeconds,
					)
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
func classifySymptomType(status string, errorMessage string, containerIssues []string) string {
	// Check container issues first (most specific)
	for _, issue := range containerIssues {
		switch issue {
		case "ImagePullBackOff", "ErrImagePull":
			return "ImagePullError"
		case "CrashLoopBackOff":
			return "CrashLoop"
		case "OOMKilled":
			return "OOMKilled"
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
		return "OOMKilled"
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
