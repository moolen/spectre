package grafana

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryInvestigateService provides deep signal inspection for the
// Narrow and Investigate stages of incident investigation.
//
// Capabilities:
//   - GetWorkloadSignals: Returns all signals for a workload with current anomaly scores
//   - GetSignalDetail: Returns detailed baseline and anomaly info for a specific signal
//   - CompareSignal: Compares signal values across time periods (current vs N hours ago)
type ObservatoryInvestigateService struct {
	graphClient     graph.Client
	queryService    QueryService
	integrationName string
	logger          *logging.Logger
}

// QueryService interface for fetching current metric values from Grafana.
// Abstracted for testability.
type QueryService interface {
	// FetchCurrentValue fetches the current value of a metric for a workload.
	// Returns the most recent value from Grafana datasource.
	FetchCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, error)

	// FetchHistoricalValue fetches a metric value from lookback duration ago.
	FetchHistoricalValue(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error)
}

// NewObservatoryInvestigateService creates a new investigation service.
func NewObservatoryInvestigateService(
	graphClient graph.Client,
	queryService QueryService,
	integrationName string,
	logger *logging.Logger,
) *ObservatoryInvestigateService {
	return &ObservatoryInvestigateService{
		graphClient:     graphClient,
		queryService:    queryService,
		integrationName: integrationName,
		logger:          logger,
	}
}

// WorkloadSignalsResult contains all signals for a workload with current anomaly scores.
// Per CONTEXT.md: "Narrow tools return ranked flat lists sorted by anomaly score"
type WorkloadSignalsResult struct {
	// Signals is the list of signals sorted by anomaly score (descending)
	Signals []SignalSummary `json:"signals"`

	// Scope identifies the workload (format: "namespace/workload")
	Scope string `json:"scope"`
}

// SignalSummary provides a minimal summary of a signal's anomaly state.
// Per CONTEXT.md: "Minimal responses - facts only, AI interprets meaning"
type SignalSummary struct {
	// MetricName is the PromQL metric name
	MetricName string `json:"metric_name"`

	// Role is the signal classification (Availability, Latency, Errors, etc.)
	Role string `json:"role"`

	// Score is the normalized anomaly score (0.0-1.0)
	Score float64 `json:"score"`

	// Confidence is the statistical confidence (0.0-1.0)
	Confidence float64 `json:"confidence"`
}

// SignalDetailResult provides detailed baseline and anomaly information for a signal.
type SignalDetailResult struct {
	// MetricName is the PromQL metric name
	MetricName string `json:"metric_name"`

	// Role is the signal classification
	Role string `json:"role"`

	// CurrentValue is the current metric value from Grafana
	CurrentValue float64 `json:"current_value"`

	// Baseline contains statistical baseline information
	Baseline BaselineStats `json:"baseline"`

	// AnomalyScore is the computed anomaly score (0.0-1.0)
	AnomalyScore float64 `json:"anomaly_score"`

	// Confidence is the statistical confidence (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// SourceDashboard is the Grafana dashboard UID that sources this signal
	SourceDashboard string `json:"source_dashboard"`

	// QualityScore is the signal quality (0.0-1.0)
	QualityScore float64 `json:"quality_score"`
}

// BaselineStats contains statistical baseline information for a signal.
type BaselineStats struct {
	// Mean is the arithmetic mean of sample values
	Mean float64 `json:"mean"`

	// StdDev is the sample standard deviation
	StdDev float64 `json:"std_dev"`

	// P50 is the 50th percentile (median)
	P50 float64 `json:"p50"`

	// P90 is the 90th percentile
	P90 float64 `json:"p90"`

	// P99 is the 99th percentile
	P99 float64 `json:"p99"`

	// SampleCount is the number of samples in the baseline
	SampleCount int `json:"sample_count"`
}

// SignalComparisonResult compares a signal across time periods.
// Per CONTEXT.md: "Compare tool compares across time only (current vs N hours/days ago)"
type SignalComparisonResult struct {
	// MetricName is the PromQL metric name
	MetricName string `json:"metric_name"`

	// CurrentValue is the current metric value
	CurrentValue float64 `json:"current_value"`

	// CurrentScore is the current anomaly score (0.0-1.0)
	CurrentScore float64 `json:"current_score"`

	// PastValue is the metric value from lookback period
	PastValue float64 `json:"past_value"`

	// PastScore is the anomaly score from lookback period
	PastScore float64 `json:"past_score"`

	// LookbackHours is the lookback period in hours
	LookbackHours int `json:"lookback_hours"`

	// ScoreDelta is the score change (Current - Past, positive = getting worse)
	ScoreDelta float64 `json:"score_delta"`
}

// DefaultLookback is the default lookback period for time comparisons.
const DefaultLookback = 24 * time.Hour

// AnomalyThreshold is the minimum anomaly score to consider a signal anomalous.
// Per CONTEXT.md: "Fixed anomaly score threshold internally"
const AnomalyThreshold = 0.5

// GetWorkloadSignals retrieves all signals for a workload with current anomaly scores.
//
// Process:
// 1. Query graph for SignalAnchors with their baselines
// 2. For each signal with sufficient baseline (SampleCount >= 10):
//   - Compute current anomaly score via ComputeAnomalyScore
//   - Include role, score, confidence
//
// 3. Return signals sorted by score descending
//
// Signals with insufficient samples (cold start) are silently skipped.
func (s *ObservatoryInvestigateService) GetWorkloadSignals(
	ctx context.Context,
	namespace, workload string,
) (*WorkloadSignalsResult, error) {
	if namespace == "" || workload == "" {
		return nil, fmt.Errorf("namespace and workload are required")
	}

	// Query graph for signals with baselines
	query := `
		MATCH (sig:SignalAnchor {
			workload_namespace: $namespace,
			workload_name: $workload,
			integration: $integration
		})
		WHERE sig.expires_at > $now
		OPTIONAL MATCH (sig)-[:HAS_BASELINE]->(b:SignalBaseline)
		RETURN sig.metric_name AS metric_name,
		       sig.role AS role,
		       sig.quality_score AS quality_score,
		       b.mean AS mean,
		       b.std_dev AS std_dev,
		       b.min AS min,
		       b.max AS max,
		       b.p50 AS p50,
		       b.p90 AS p90,
		       b.p99 AS p99,
		       b.sample_count AS sample_count
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace":   namespace,
			"workload":    workload,
			"integration": s.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query signals: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var signals []SignalSummary
	for _, row := range result.Rows {
		// Extract signal fields
		metricName := s.extractString(row, colIdx, "metric_name")
		role := s.extractString(row, colIdx, "role")
		qualityScore := s.extractFloat64(row, colIdx, "quality_score")

		// Check if baseline exists (sample_count not nil)
		sampleCountIdx, hasSampleCount := colIdx["sample_count"]
		if !hasSampleCount || sampleCountIdx >= len(row) || row[sampleCountIdx] == nil {
			// No baseline - skip signal (cold start)
			s.logger.Debug("Skipping signal %s: no baseline", metricName)
			continue
		}

		// Build baseline
		baseline := SignalBaseline{
			Mean:        s.extractFloat64(row, colIdx, "mean"),
			StdDev:      s.extractFloat64(row, colIdx, "std_dev"),
			Min:         s.extractFloat64(row, colIdx, "min"),
			Max:         s.extractFloat64(row, colIdx, "max"),
			P50:         s.extractFloat64(row, colIdx, "p50"),
			P90:         s.extractFloat64(row, colIdx, "p90"),
			P99:         s.extractFloat64(row, colIdx, "p99"),
			SampleCount: s.extractInt(row, colIdx, "sample_count"),
		}

		// Use baseline mean as current value proxy
		// TODO: In production, fetch current value from Grafana
		currentValue := baseline.Mean

		// Compute anomaly score
		score, err := ComputeAnomalyScore(currentValue, baseline, qualityScore)
		if err != nil {
			var insufficientErr *InsufficientSamplesError
			if errors.As(err, &insufficientErr) {
				s.logger.Debug("Skipping signal %s: %v", metricName, err)
				continue // Skip cold-start signals
			}
			return nil, fmt.Errorf("compute anomaly score for %s: %w", metricName, err)
		}

		signals = append(signals, SignalSummary{
			MetricName: metricName,
			Role:       role,
			Score:      score.Score,
			Confidence: score.Confidence,
		})
	}

	// Sort by score descending, then by confidence descending as tiebreaker
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].Score != signals[j].Score {
			return signals[i].Score > signals[j].Score
		}
		return signals[i].Confidence > signals[j].Confidence
	})

	return &WorkloadSignalsResult{
		Signals: signals,
		Scope:   fmt.Sprintf("%s/%s", namespace, workload),
	}, nil
}

// GetSignalDetail retrieves detailed baseline and anomaly information for a specific signal.
//
// Process:
// 1. Query graph for specific SignalAnchor with baseline
// 2. Fetch current metric value from Grafana via queryService
// 3. Compute anomaly score
// 4. Return detailed response with baseline stats, current value, score, confidence
//
// Returns error if signal not found or baseline unavailable.
func (s *ObservatoryInvestigateService) GetSignalDetail(
	ctx context.Context,
	namespace, workload, metricName string,
) (*SignalDetailResult, error) {
	if namespace == "" || workload == "" || metricName == "" {
		return nil, fmt.Errorf("namespace, workload, and metric_name are required")
	}

	// Query for specific SignalAnchor with baseline and dashboard source
	query := `
		MATCH (sig:SignalAnchor {
			metric_name: $metric_name,
			workload_namespace: $namespace,
			workload_name: $workload,
			integration: $integration
		})
		WHERE sig.expires_at > $now
		OPTIONAL MATCH (sig)-[:HAS_BASELINE]->(b:SignalBaseline)
		OPTIONAL MATCH (sig)-[:EXTRACTED_FROM]->(q:Query)-[:BELONGS_TO]->(p:Panel)-[:BELONGS_TO]->(d:Dashboard)
		RETURN sig.role AS role,
		       sig.quality_score AS quality_score,
		       d.uid AS dashboard_uid,
		       b.mean AS mean,
		       b.std_dev AS std_dev,
		       b.min AS min,
		       b.max AS max,
		       b.p50 AS p50,
		       b.p90 AS p90,
		       b.p99 AS p99,
		       b.sample_count AS sample_count
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name": metricName,
			"namespace":   namespace,
			"workload":    workload,
			"integration": s.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query signal: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("signal not found: %s/%s/%s", namespace, workload, metricName)
	}

	row := result.Rows[0]

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	// Extract signal fields
	role := s.extractString(row, colIdx, "role")
	qualityScore := s.extractFloat64(row, colIdx, "quality_score")
	dashboardUID := s.extractString(row, colIdx, "dashboard_uid")

	// Check if baseline exists
	sampleCountIdx, hasSampleCount := colIdx["sample_count"]
	if !hasSampleCount || sampleCountIdx >= len(row) || row[sampleCountIdx] == nil {
		return nil, fmt.Errorf("signal %s has no baseline (cold start)", metricName)
	}

	// Build baseline
	baseline := SignalBaseline{
		Mean:        s.extractFloat64(row, colIdx, "mean"),
		StdDev:      s.extractFloat64(row, colIdx, "std_dev"),
		Min:         s.extractFloat64(row, colIdx, "min"),
		Max:         s.extractFloat64(row, colIdx, "max"),
		P50:         s.extractFloat64(row, colIdx, "p50"),
		P90:         s.extractFloat64(row, colIdx, "p90"),
		P99:         s.extractFloat64(row, colIdx, "p99"),
		SampleCount: s.extractInt(row, colIdx, "sample_count"),
	}

	// Fetch current value from Grafana
	var currentValue float64
	if s.queryService != nil {
		currentValue, err = s.queryService.FetchCurrentValue(ctx, metricName, namespace, workload)
		if err != nil {
			// Log but don't fail - use baseline mean as fallback
			s.logger.Debug("Failed to fetch current value for %s: %v, using baseline mean", metricName, err)
			currentValue = baseline.Mean
		}
	} else {
		// No query service - use baseline mean
		currentValue = baseline.Mean
	}

	// Compute anomaly score
	score, err := ComputeAnomalyScore(currentValue, baseline, qualityScore)
	if err != nil {
		return nil, fmt.Errorf("compute anomaly score: %w", err)
	}

	return &SignalDetailResult{
		MetricName:   metricName,
		Role:         role,
		CurrentValue: currentValue,
		Baseline: BaselineStats{
			Mean:        baseline.Mean,
			StdDev:      baseline.StdDev,
			P50:         baseline.P50,
			P90:         baseline.P90,
			P99:         baseline.P99,
			SampleCount: baseline.SampleCount,
		},
		AnomalyScore:    score.Score,
		Confidence:      score.Confidence,
		SourceDashboard: dashboardUID,
		QualityScore:    qualityScore,
	}, nil
}

// CompareSignal compares signal values across time periods.
//
// Per CONTEXT.md: "Compare tool compares across time only (current vs N hours/days ago)"
//
// Process:
// 1. Fetch current value and historical value (lookback ago) from Grafana
// 2. Compare both against baseline to get anomaly scores
// 3. Return comparison showing score change
//
// Default lookback: 24 hours
func (s *ObservatoryInvestigateService) CompareSignal(
	ctx context.Context,
	namespace, workload, metricName string,
	lookback time.Duration,
) (*SignalComparisonResult, error) {
	if namespace == "" || workload == "" || metricName == "" {
		return nil, fmt.Errorf("namespace, workload, and metric_name are required")
	}

	// Apply default lookback if not specified
	if lookback == 0 {
		lookback = DefaultLookback
	}

	// First get the signal detail to get baseline
	detail, err := s.GetSignalDetail(ctx, namespace, workload, metricName)
	if err != nil {
		return nil, fmt.Errorf("get signal detail: %w", err)
	}

	// Build baseline from detail for scoring
	baseline := SignalBaseline{
		Mean:        detail.Baseline.Mean,
		StdDev:      detail.Baseline.StdDev,
		P50:         detail.Baseline.P50,
		P90:         detail.Baseline.P90,
		P99:         detail.Baseline.P99,
		SampleCount: detail.Baseline.SampleCount,
	}

	currentValue := detail.CurrentValue

	// Fetch historical value
	var pastValue float64
	if s.queryService != nil {
		pastValue, err = s.queryService.FetchHistoricalValue(ctx, metricName, namespace, workload, lookback)
		if err != nil {
			// Log but don't fail - use baseline mean as fallback
			s.logger.Debug("Failed to fetch historical value for %s: %v, using baseline mean", metricName, err)
			pastValue = baseline.Mean
		}
	} else {
		// No query service - use baseline mean
		pastValue = baseline.Mean
	}

	// Compute current anomaly score (already computed in detail)
	currentScore := detail.AnomalyScore

	// Compute past anomaly score
	pastScoreResult, err := ComputeAnomalyScore(pastValue, baseline, detail.QualityScore)
	if err != nil {
		return nil, fmt.Errorf("compute past anomaly score: %w", err)
	}
	pastScore := pastScoreResult.Score

	// Calculate score delta (positive = getting worse)
	scoreDelta := currentScore - pastScore

	return &SignalComparisonResult{
		MetricName:    metricName,
		CurrentValue:  currentValue,
		CurrentScore:  currentScore,
		PastValue:     pastValue,
		PastScore:     pastScore,
		LookbackHours: int(lookback.Hours()),
		ScoreDelta:    scoreDelta,
	}, nil
}

// Helper methods for extracting values from query results

func (s *ObservatoryInvestigateService) extractString(row []interface{}, colIdx map[string]int, col string) string {
	if idx, ok := colIdx[col]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			return v
		}
	}
	return ""
}

func (s *ObservatoryInvestigateService) extractFloat64(row []interface{}, colIdx map[string]int, col string) float64 {
	if idx, ok := colIdx[col]; ok && idx < len(row) {
		return parseFloat64(row[idx])
	}
	return 0
}

func (s *ObservatoryInvestigateService) extractInt(row []interface{}, colIdx map[string]int, col string) int {
	if idx, ok := colIdx[col]; ok && idx < len(row) {
		return parseInt(row[idx])
	}
	return 0
}
