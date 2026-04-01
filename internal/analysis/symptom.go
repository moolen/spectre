package analysis

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	analyzerpkg "github.com/moolen/spectre/internal/analyzer"
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
	resource, err := a.store.GetResource(ctx, resourceUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource: %w", err)
	}
	if resource == nil {
		return nil, fmt.Errorf("resource %s not found", resourceUID)
	}

	const tolerance = int64(300_000_000_000) // 5 minutes
	window := analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp + tolerance,
		LookbackNs:         tolerance * 2,
	}

	eventsByUID, err := a.store.GetChangeEvents(ctx, []string{resourceUID}, window)
	if err != nil {
		return nil, fmt.Errorf("failed to query failure event: %w", err)
	}

	event, found := closestEventInTolerance(convertStoreChangeEventList(eventsByUID[resourceUID]), failureTimestamp, tolerance)
	if !found {
		if resource.FirstSeen != 0 {
			firstEventTS := resource.FirstSeen
			firstEventTime := time.Unix(0, firstEventTS)
			providedTime := time.Unix(0, failureTimestamp)
			diffSeconds := (firstEventTS - failureTimestamp) / 1_000_000_000

			if diffSeconds > 300 {
				return nil, &ErrNoChangeEventInRange{
					RequestedTimestamp:  failureTimestamp,
					RequestedTime:       providedTime,
					FirstEventTimestamp: firstEventTS,
					FirstEventTime:      firstEventTime,
					DiffSeconds:         diffSeconds,
					SuggestedTimestamp:  firstEventTS,
					TimestampTooEarly:   true,
				}
			}
			if diffSeconds < -300 {
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

		return nil, fmt.Errorf("no ChangeEvent found for resource %s at timestamp %d", resourceUID, failureTimestamp)
	}

	errorMessage, containerIssues := a.getObservedErrorDetails(ctx, resource, event.Timestamp.UnixNano(), tolerance)
	if errorMessage == "" && len(event.Data) > 0 {
		errors := analyzerpkg.InferErrorMessages(resource.Kind, event.Data, event.Status)
		if len(errors) > 0 {
			errorMessage = strings.Join(errors, "; ")
		}
	}
	if len(containerIssues) == 0 {
		containerIssues = inferContainerIssues(errorMessage)
	}

	symptomType := classifySymptomType(event.Status, errorMessage, containerIssues)

	return &ObservedSymptom{
		Resource: SymptomResource{
			UID:       resource.UID,
			Kind:      resource.Kind,
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
		Status:       event.Status,
		ErrorMessage: errorMessage,
		ObservedAt:   event.Timestamp,
		SymptomType:  symptomType,
	}, nil
}

func closestEventInTolerance(events []ChangeEventInfo, failureTimestamp, tolerance int64) (ChangeEventInfo, bool) {
	var (
		best     ChangeEventInfo
		bestDiff int64 = math.MaxInt64
		found    bool
	)

	lowerBound := failureTimestamp - tolerance
	upperBound := failureTimestamp + tolerance
	for _, event := range events {
		timestamp := event.Timestamp.UnixNano()
		if timestamp < lowerBound || timestamp > upperBound {
			continue
		}

		diff := timestamp - failureTimestamp
		if diff < 0 {
			diff = -diff
		}
		if !found || diff < bestDiff {
			best = event
			bestDiff = diff
			found = true
		}
	}

	return best, found
}

func (a *RootCauseAnalyzer) getObservedErrorDetails(
	ctx context.Context,
	resource *graph.ResourceIdentity,
	timestampNs int64,
	lookbackNs int64,
) (string, []string) {
	if resource.Namespace == "" {
		return "", nil
	}

	graphData, err := a.store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   resource.Namespace,
		TimestampNs: timestampNs,
		LookbackNs:  lookbackNs,
		Limit:       500,
	})
	if err != nil || graphData == nil {
		return "", nil
	}

	for _, node := range graphData.Graph.Nodes {
		if node.UID == resource.UID && node.LatestEvent != nil {
			return node.LatestEvent.ErrorMessage, append([]string(nil), node.LatestEvent.ContainerIssues...)
		}
	}

	return "", nil
}

func inferContainerIssues(errorMessage string) []string {
	lower := strings.ToLower(errorMessage)
	issues := []string{}

	switch {
	case strings.Contains(lower, "imagepullbackoff"):
		issues = append(issues, "ImagePullBackOff")
	case strings.Contains(lower, "errimagepull"):
		issues = append(issues, "ErrImagePull")
	}

	if strings.Contains(lower, "crashloopbackoff") || strings.Contains(lower, "back-off restarting failed container") {
		issues = append(issues, "CrashLoopBackOff")
	}
	if strings.Contains(lower, "oomkilled") || strings.Contains(lower, "out of memory") {
		issues = append(issues, reasonOOMKilled)
	}
	if strings.Contains(lower, "containercreating") {
		issues = append(issues, "ContainerCreating")
	}

	return issues
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

	switch status {
	case "Error":
		return "Error"
	case "Warning":
		return "Warning"
	case "Terminating":
		return "Terminating"
	case "Pending":
		if strings.Contains(errorLower, "node") || strings.Contains(errorLower, "pending") {
			return "SchedulingFailure"
		}
		return "Pending"
	default:
		return "Unknown"
	}
}
