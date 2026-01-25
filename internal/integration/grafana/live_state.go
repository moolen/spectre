package grafana

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// alertStatePoint represents a single point in time where an alert was in a specific state.
// Used internally for parsing ALERTS metric data before converting to StateTransitions.
type alertStatePoint struct {
	timestamp time.Time
	state     string // "firing", "pending"
}

// LiveStateProvider fetches alert state history directly from Prometheus/Grafana
// by querying the ALERTS metric, bypassing the need for synced STATE_TRANSITION edges.
type LiveStateProvider struct {
	client          GrafanaClientInterface
	datasourceUID   string // Prometheus datasource UID
	integrationName string
	logger          *logging.Logger
}

// NewLiveStateProvider creates a new LiveStateProvider instance.
// datasourceUID should be the UID of the Prometheus datasource in Grafana.
func NewLiveStateProvider(
	client GrafanaClientInterface,
	datasourceUID string,
	integrationName string,
	logger *logging.Logger,
) *LiveStateProvider {
	return &LiveStateProvider{
		client:          client,
		datasourceUID:   datasourceUID,
		integrationName: integrationName,
		logger:          logger,
	}
}

// FetchLiveStateTransitions queries the ALERTS metric to get real-time state history.
// This provides immediate visibility into alert state changes without sync latency.
//
// The ALERTS metric format: ALERTS{alertname="...", alertstate="firing|pending", ...}
// - Value 1 = alert is in that state
// - No data = alert is normal/inactive
//
// Parameters:
//   - ctx: context for cancellation
//   - alertName: the alertname label value (from Grafana alert rule title)
//   - startTime: start of time window
//   - endTime: end of time window
//
// Returns:
//   - transitions: slice of state transitions derived from metric data
//   - error: query or parsing errors
func (p *LiveStateProvider) FetchLiveStateTransitions(
	ctx context.Context,
	alertName string,
	startTime time.Time,
	endTime time.Time,
) ([]StateTransition, error) {
	p.logger.Debug("Fetching live state transitions for alert %s from %s to %s",
		alertName, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Query ALERTS metric for both firing and pending states
	// We query both states to capture the full picture
	expr := fmt.Sprintf(`ALERTS{alertname="%s"}`, alertName)

	// Convert times to epoch milliseconds (Grafana format)
	fromMs := strconv.FormatInt(startTime.UnixMilli(), 10)
	toMs := strconv.FormatInt(endTime.UnixMilli(), 10)

	// Execute query via Grafana
	resp, err := p.client.QueryDataSource(ctx, p.datasourceUID, expr, fromMs, toMs, nil)
	if err != nil {
		return nil, fmt.Errorf("query ALERTS metric: %w", err)
	}

	// Parse response into state transitions
	transitions, err := p.parseAlertsResponse(resp, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("parse ALERTS response: %w", err)
	}

	p.logger.Debug("Found %d state transitions for alert %s", len(transitions), alertName)
	return transitions, nil
}

// parseAlertsResponse converts Grafana query response into StateTransitions.
// The ALERTS metric produces time series with alertstate label indicating firing/pending.
// We detect state changes by looking at when series start/stop having data.
func (p *LiveStateProvider) parseAlertsResponse(
	resp *QueryResponse,
	startTime time.Time,
	endTime time.Time,
) ([]StateTransition, error) {
	if resp == nil {
		return nil, nil
	}

	// Collect all state points from all frames
	var allPoints []alertStatePoint

	// Process each result (should be one for refId "A")
	for _, result := range resp.Results {
		if result.Error != "" {
			return nil, fmt.Errorf("query error: %s", result.Error)
		}

		// Process each frame (one per label combination)
		for _, frame := range result.Frames {
			// Extract alertstate from schema labels
			alertState := ""
			for _, field := range frame.Schema.Fields {
				if field.Labels != nil {
					if state, ok := field.Labels["alertstate"]; ok {
						alertState = state
						break
					}
				}
			}

			if alertState == "" {
				// Try to get from schema name which might contain labels
				p.logger.Debug("No alertstate label found in frame, skipping")
				continue
			}

			// Parse data values
			// DataFrame.Data.Values format: [[timestamps...], [values...]]
			if len(frame.Data.Values) < 2 {
				continue
			}

			timestamps := frame.Data.Values[0]
			values := frame.Data.Values[1]

			for i := 0; i < len(timestamps) && i < len(values); i++ {
				// Parse timestamp (can be float64 epoch ms or int64)
				var ts time.Time
				switch t := timestamps[i].(type) {
				case float64:
					ts = time.UnixMilli(int64(t))
				case int64:
					ts = time.UnixMilli(t)
				case int:
					ts = time.UnixMilli(int64(t))
				default:
					p.logger.Debug("Unexpected timestamp type: %T", timestamps[i])
					continue
				}

				// Parse value (should be 1 when alert is active)
				var val float64
				switch v := values[i].(type) {
				case float64:
					val = v
				case int64:
					val = float64(v)
				case int:
					val = float64(v)
				default:
					continue
				}

				// Only record points where value is 1 (alert active)
				if val == 1 {
					allPoints = append(allPoints, alertStatePoint{
						timestamp: ts,
						state:     alertState,
					})
				}
			}
		}
	}

	// Convert state points to transitions
	return p.deriveTransitions(allPoints, startTime, endTime), nil
}

// deriveTransitions converts a series of state points into state transitions.
// It detects when the alert state changes between normal, pending, and firing.
func (p *LiveStateProvider) deriveTransitions(
	points []alertStatePoint,
	startTime time.Time,
	endTime time.Time,
) []StateTransition {
	if len(points) == 0 {
		return nil
	}

	// Sort by timestamp
	sort.Slice(points, func(i, j int) bool {
		return points[i].timestamp.Before(points[j].timestamp)
	})

	var transitions []StateTransition
	lastState := "normal" // Assume normal at start if no data

	// Group points by timestamp buckets (within same second = same state)
	// This handles cases where we might have both firing and pending at same time
	type bucket struct {
		timestamp time.Time
		states    map[string]bool
	}
	var buckets []bucket

	for _, pt := range points {
		// Round to nearest second for bucketing
		bucketTime := pt.timestamp.Truncate(time.Second)

		if len(buckets) == 0 || !buckets[len(buckets)-1].timestamp.Equal(bucketTime) {
			buckets = append(buckets, bucket{
				timestamp: bucketTime,
				states:    make(map[string]bool),
			})
		}
		buckets[len(buckets)-1].states[pt.state] = true
	}

	// Process buckets to find transitions
	for i, b := range buckets {
		// Determine effective state (firing takes precedence over pending)
		var currentState string
		if b.states["firing"] {
			currentState = "firing"
		} else if b.states["pending"] {
			currentState = "pending"
		} else {
			currentState = "normal"
		}

		// Check for gaps between buckets (indicates return to normal)
		if i > 0 {
			prevBucket := buckets[i-1]
			gap := b.timestamp.Sub(prevBucket.timestamp)

			// If gap is larger than expected step interval (assume ~1min), there was a normal period
			// Grafana typically uses 15s-1m evaluation intervals
			if gap > 2*time.Minute && lastState != "normal" {
				// Insert transition to normal at midpoint of gap
				normalTime := prevBucket.timestamp.Add(time.Minute)
				transitions = append(transitions, StateTransition{
					FromState: lastState,
					ToState:   "normal",
					Timestamp: normalTime,
				})
				lastState = "normal"
			}
		}

		// Record transition if state changed
		if currentState != lastState {
			transitions = append(transitions, StateTransition{
				FromState: lastState,
				ToState:   currentState,
				Timestamp: b.timestamp,
			})
			lastState = currentState
		}
	}

	// If the last known state was not normal and we're past the last data point,
	// check if we should add a transition back to normal
	if len(buckets) > 0 && lastState != "normal" {
		lastBucket := buckets[len(buckets)-1]
		if endTime.Sub(lastBucket.timestamp) > 2*time.Minute {
			// Add transition to normal
			normalTime := lastBucket.timestamp.Add(time.Minute)
			if normalTime.Before(endTime) {
				transitions = append(transitions, StateTransition{
					FromState: lastState,
					ToState:   "normal",
					Timestamp: normalTime,
				})
			}
		}
	}

	return transitions
}
