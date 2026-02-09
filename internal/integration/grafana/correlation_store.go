package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// CorrelationObservation records a single evaluation of signal-alert correlation
type CorrelationObservation struct {
	Timestamp      time.Time
	WasSignificant bool
	Stats          graph.SignalCorrelationStats
}

// SignalAnchorKey identifies a SignalAnchor node
type SignalAnchorKey struct {
	MetricName        string
	WorkloadNamespace string
	WorkloadName      string
}

// CorrelationStore manages CORRELATES_WITH edges and aggregate scores.
type CorrelationStore struct {
	graphClient     graph.Client
	integrationName string
	decayPeriod     time.Duration
	logger          *logging.Logger
}

// NewCorrelationStore creates a new CorrelationStore.
func NewCorrelationStore(
	graphClient graph.Client,
	integrationName string,
	decayPeriod time.Duration,
	logger *logging.Logger,
) *CorrelationStore {
	return &CorrelationStore{
		graphClient:     graphClient,
		integrationName: integrationName,
		decayPeriod:     decayPeriod,
		logger:          logger,
	}
}

// RecordObservation adds or updates a correlation observation between a signal and alert.
// Updates the CORRELATES_WITH edge with new statistics and recomputes the aggregate score.
func (s *CorrelationStore) RecordObservation(
	ctx context.Context,
	signalKey SignalAnchorKey,
	alertUID string,
	workloadUID string,
	workloadName string,
	namespace string,
	observation CorrelationObservation,
) error {
	now := time.Now().UnixNano()

	// Convert stats to JSON for storage
	statsJSON, err := json.Marshal(observation.Stats)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	// Calculate score contribution (1.0 for significant, 0.0 for not)
	scoreContribution := 0.0
	if observation.WasSignificant {
		scoreContribution = 1.0
	}

	significantInt := 0
	if observation.WasSignificant {
		significantInt = 1
	}

	query := `
MATCH (s:SignalAnchor {
    metric_name: $metricName,
    workload_namespace: $workloadNamespace,
    workload_name: $workloadName
})
MATCH (a:Alert {uid: $alertUID, integration: $integration})
MERGE (s)-[c:CORRELATES_WITH {workload_uid: $workloadUID}]->(a)
ON CREATE SET
    c.workload_name = $workloadName,
    c.namespace = $namespace,
    c.transitions_evaluated = 1,
    c.significant_changes = $significantInt,
    c.stats = $stats,
    c.correlation_score = $scoreContribution,
    c.first_evaluated = $now,
    c.last_evaluated = $now,
    c.last_significant = CASE WHEN $wasSignificant THEN $now ELSE 0 END
ON MATCH SET
    c.transitions_evaluated = c.transitions_evaluated + 1,
    c.significant_changes = c.significant_changes + $significantInt,
    c.stats = $stats,
    c.last_evaluated = $now,
    c.last_significant = CASE WHEN $wasSignificant THEN $now ELSE c.last_significant END
RETURN c.transitions_evaluated AS total, c.significant_changes AS significant
`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metricName":        signalKey.MetricName,
			"workloadNamespace": signalKey.WorkloadNamespace,
			"workloadName":      signalKey.WorkloadName,
			"alertUID":          alertUID,
			"integration":       s.integrationName,
			"workloadUID":       workloadUID,
			"namespace":         namespace,
			"stats":             string(statsJSON),
			"scoreContribution": scoreContribution,
			"significantInt":    significantInt,
			"wasSignificant":    observation.WasSignificant,
			"now":               now,
		},
		Timeout: 10000,
	})
	if err != nil {
		return fmt.Errorf("failed to record observation: %w", err)
	}

	// Update the correlation score based on significant/total ratio
	if len(result.Rows) > 0 && len(result.Rows[0]) >= 2 {
		total, _ := result.Rows[0][0].(int64)
		significant, _ := result.Rows[0][1].(int64)
		if total > 0 {
			newScore := float64(significant) / float64(total)
			if err := s.updateCorrelationScore(ctx, signalKey, alertUID, workloadUID, newScore); err != nil {
				s.logger.Warn("Failed to update correlation score: %v", err)
			}
		}
	}

	return nil
}

// updateCorrelationScore updates the correlation_score on the edge.
func (s *CorrelationStore) updateCorrelationScore(
	ctx context.Context,
	signalKey SignalAnchorKey,
	alertUID string,
	workloadUID string,
	score float64,
) error {
	query := `
MATCH (s:SignalAnchor {
    metric_name: $metricName,
    workload_namespace: $workloadNamespace,
    workload_name: $workloadName
})-[c:CORRELATES_WITH {workload_uid: $workloadUID}]->(a:Alert {uid: $alertUID})
SET c.correlation_score = $score
`

	_, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metricName":        signalKey.MetricName,
			"workloadNamespace": signalKey.WorkloadNamespace,
			"workloadName":      signalKey.WorkloadName,
			"alertUID":          alertUID,
			"workloadUID":       workloadUID,
			"score":             score,
		},
		Timeout: 5000,
	})
	return err
}

// UpdateSignalAnchorAggregateScore recomputes the aggregate HistoricalCorrelationScore
// on a SignalAnchor by combining scores from all its CORRELATES_WITH edges.
func (s *CorrelationStore) UpdateSignalAnchorAggregateScore(ctx context.Context, signalKey SignalAnchorKey) error {
	now := time.Now().UnixNano()
	decayCutoff := time.Now().Add(-s.decayPeriod).UnixNano()
	decayPeriodNanos := float64(s.decayPeriod.Nanoseconds())

	query := `
MATCH (s:SignalAnchor {
    metric_name: $metricName,
    workload_namespace: $workloadNamespace,
    workload_name: $workloadName
})
OPTIONAL MATCH (s)-[c:CORRELATES_WITH]->(:Alert)
WHERE c.last_evaluated >= $decayCutoff
WITH s,
     CASE WHEN c IS NOT NULL THEN
         sum(c.correlation_score * (1.0 - (toFloat($now - c.last_evaluated) / $decayPeriodNanos)))
     ELSE 0 END AS weightedScore,
     CASE WHEN c IS NOT NULL THEN
         sum(1.0 - (toFloat($now - c.last_evaluated) / $decayPeriodNanos))
     ELSE 0 END AS totalWeight
SET s.historical_correlation_score = CASE
    WHEN totalWeight > 0 THEN weightedScore / totalWeight
    ELSE 0
END,
    s.correlation_evaluated_at = $now
RETURN s.historical_correlation_score AS score
`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metricName":        signalKey.MetricName,
			"workloadNamespace": signalKey.WorkloadNamespace,
			"workloadName":      signalKey.WorkloadName,
			"decayCutoff":       decayCutoff,
			"now":               now,
			"decayPeriodNanos":  decayPeriodNanos,
		},
		Timeout: 10000,
	})
	if err != nil {
		return fmt.Errorf("failed to update aggregate score: %w", err)
	}

	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		if score, ok := result.Rows[0][0].(float64); ok {
			s.logger.Debug("Updated aggregate score for %s/%s/%s: %.3f",
				signalKey.MetricName, signalKey.WorkloadNamespace, signalKey.WorkloadName, score)
		}
	}

	return nil
}

// ListCorrelationsForAlert returns all CORRELATES_WITH edges for an alert.
func (s *CorrelationStore) ListCorrelationsForAlert(ctx context.Context, alertUID string) ([]CorrelationEdgeInfo, error) {
	query := `
MATCH (s:SignalAnchor)-[c:CORRELATES_WITH]->(a:Alert {uid: $alertUID, integration: $integration})
RETURN s.metric_name AS metricName,
       s.workload_namespace AS workloadNamespace,
       s.workload_name AS workloadName,
       c.correlation_score AS score,
       c.transitions_evaluated AS evaluated,
       c.significant_changes AS significant
ORDER BY c.correlation_score DESC
`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"alertUID":    alertUID,
			"integration": s.integrationName,
		},
		Timeout: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list correlations: %w", err)
	}

	var correlations []CorrelationEdgeInfo
	for _, row := range result.Rows {
		if len(row) < 6 {
			continue
		}

		metricName, _ := row[0].(string)
		workloadNamespace, _ := row[1].(string)
		workloadName, _ := row[2].(string)
		score, _ := row[3].(float64)
		evaluated, _ := row[4].(int64)
		significant, _ := row[5].(int64)

		correlations = append(correlations, CorrelationEdgeInfo{
			MetricName:        metricName,
			WorkloadNamespace: workloadNamespace,
			WorkloadName:      workloadName,
			Score:             score,
			Evaluated:         int(evaluated),
			Significant:       int(significant),
		})
	}

	return correlations, nil
}

// ListUncorrelatedAlerts returns alerts that have transitions but no CORRELATES_WITH edges.
// This is used for reconciliation to find new alerts that need processing.
func (s *CorrelationStore) ListUncorrelatedAlerts(ctx context.Context, limit int) ([]string, error) {
	query := `
MATCH (a:Alert {integration: $integration})-[:STATE_TRANSITION]->(a)
WHERE NOT EXISTS {
    (:SignalAnchor)-[:CORRELATES_WITH]->(a)
}
RETURN DISTINCT a.uid AS uid
LIMIT $limit
`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": s.integrationName,
			"limit":       limit,
		},
		Timeout: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list uncorrelated alerts: %w", err)
	}

	var alertUIDs []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if uid, ok := row[0].(string); ok {
				alertUIDs = append(alertUIDs, uid)
			}
		}
	}

	return alertUIDs, nil
}

// CorrelationEdgeInfo holds summary information about a CORRELATES_WITH edge.
type CorrelationEdgeInfo struct {
	MetricName        string
	WorkloadNamespace string
	WorkloadName      string
	Score             float64
	Evaluated         int
	Significant       int
}
