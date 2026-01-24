package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// FetchStateTransitions retrieves state transitions for an alert from the graph
// within a specified time range. Queries STATE_TRANSITION edges with temporal filtering.
//
// Returns an empty slice (not error) if no transitions found, which is valid for new alerts.
//
// Parameters:
//   - ctx: context for cancellation
//   - graphClient: graph client for executing Cypher queries
//   - alertUID: unique identifier of the alert
//   - integrationName: name of the Grafana integration
//   - startTime: start of time window (inclusive)
//   - endTime: end of time window (inclusive)
//
// Returns:
//   - transitions: slice of state transitions sorted chronologically
//   - error: graph client errors or timestamp parsing failures
func FetchStateTransitions(
	ctx context.Context,
	graphClient graph.Client,
	alertUID string,
	integrationName string,
	startTime time.Time,
	endTime time.Time,
) ([]StateTransition, error) {
	logger := logging.GetLogger("grafana.transitions")

	// Convert times to UTC and format as RFC3339 (Phase 21-01 pattern)
	startTimeUTC := startTime.UTC().Format(time.RFC3339)
	endTimeUTC := endTime.UTC().Format(time.RFC3339)
	nowUTC := time.Now().UTC().Format(time.RFC3339)

	// Cypher query to fetch state transitions with temporal filtering
	// Uses self-edge pattern: (Alert)-[STATE_TRANSITION]->(Alert)
	// Filters by expires_at to respect 7-day TTL (Phase 21-01 decision)
	query := `
MATCH (a:Alert {uid: $uid, integration: $integration})-[t:STATE_TRANSITION]->(a)
WHERE t.timestamp >= $startTime
  AND t.timestamp <= $endTime
  AND t.expires_at > $now
RETURN t.from_state AS from_state,
       t.to_state AS to_state,
       t.timestamp AS timestamp
ORDER BY t.timestamp ASC
`

	result, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid":         alertUID,
			"integration": integrationName,
			"startTime":   startTimeUTC,
			"endTime":     endTimeUTC,
			"now":         nowUTC,
		},
		Timeout: 5000, // 5 seconds
	})
	if err != nil {
		return nil, fmt.Errorf("graph query failed: %w", err)
	}

	// Parse results into StateTransition structs
	transitions := make([]StateTransition, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 3 {
			logger.Warn("Skipping row with insufficient columns: %v", row)
			continue
		}

		// Extract fields from row
		fromState, ok := row[0].(string)
		if !ok {
			logger.Warn("Skipping row with invalid from_state type: %v", row[0])
			continue
		}

		toState, ok := row[1].(string)
		if !ok {
			logger.Warn("Skipping row with invalid to_state type: %v", row[1])
			continue
		}

		timestampStr, ok := row[2].(string)
		if !ok {
			logger.Warn("Skipping row with invalid timestamp type: %v", row[2])
			continue
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			logger.Warn("Skipping row with unparseable timestamp %s: %v", timestampStr, err)
			continue
		}

		transitions = append(transitions, StateTransition{
			FromState: fromState,
			ToState:   toState,
			Timestamp: timestamp,
		})
	}

	logger.Debug("Fetched %d state transitions for alert %s from %s to %s",
		len(transitions), alertUID, startTimeUTC, endTimeUTC)

	// Return empty slice if no transitions (valid for new alerts)
	return transitions, nil
}
