package observatory

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// InvestigateService provides deep signal inspection for the
// Narrow and Investigate stages of incident investigation.
type InvestigateService struct {
	registry *Registry
}

// NewInvestigateService creates a new investigation service.
func NewInvestigateService(registry *Registry) *InvestigateService {
	return &InvestigateService{
		registry: registry,
	}
}

// WorkloadSignalsResult contains all signals for a workload with current anomaly scores.
type WorkloadSignalsResult struct {
	Signals []SignalSummary `json:"signals"`
	Scope   string          `json:"scope"`
}

// SignalSummary provides a minimal summary of a signal's anomaly state.
type SignalSummary struct {
	MetricName   string  `json:"metric_name"`
	Role         string  `json:"role"`
	Score        float64 `json:"score"`
	Confidence   float64 `json:"confidence"`
	QualityScore float64 `json:"quality_score"`
}

// SignalDetailResult provides detailed baseline and anomaly information for a signal.
type SignalDetailResult struct {
	MetricName      string        `json:"metric_name"`
	Role            string        `json:"role"`
	CurrentValue    float64       `json:"current_value"`
	Baseline        BaselineStats `json:"baseline"`
	AnomalyScore    float64       `json:"anomaly_score"`
	Confidence      float64       `json:"confidence"`
	SourceProvider  string        `json:"source_provider"`
	SourceRef       string        `json:"source_ref"`
	QualityScore    float64       `json:"quality_score"`
}

// BaselineStats contains statistical baseline information for a signal.
type BaselineStats struct {
	Mean        float64 `json:"mean"`
	StdDev      float64 `json:"std_dev"`
	P50         float64 `json:"p50"`
	P90         float64 `json:"p90"`
	P99         float64 `json:"p99"`
	SampleCount int     `json:"sample_count"`
}

// SignalComparisonResult compares a signal across time periods.
type SignalComparisonResult struct {
	MetricName    string  `json:"metric_name"`
	CurrentValue  float64 `json:"current_value"`
	CurrentScore  float64 `json:"current_score"`
	PastValue     float64 `json:"past_value"`
	PastScore     float64 `json:"past_score"`
	LookbackHours int     `json:"lookback_hours"`
	ScoreDelta    float64 `json:"score_delta"`
}

// DefaultLookback is the default lookback period for time comparisons.
const DefaultLookback = 24 * time.Hour

// GetWorkloadSignals retrieves all signals for a workload with current anomaly scores.
//
// Process:
// 1. Query registry for SignalAnchors
// 2. For each signal with sufficient baseline, compute current anomaly score
// 3. Return signals sorted by score descending
func (s *InvestigateService) GetWorkloadSignals(
	ctx context.Context,
	namespace, workload string,
) (*WorkloadSignalsResult, error) {
	if namespace == "" || workload == "" {
		return nil, fmt.Errorf("namespace and workload are required")
	}

	// Query registry for signals
	signals, err := s.registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace:    namespace,
		WorkloadName: workload,
	})
	if err != nil {
		return nil, fmt.Errorf("list signals: %w", err)
	}

	var summaries []SignalSummary
	for _, signal := range signals {
		// Get baseline
		baseline, err := s.registry.GetSignalBaseline(ctx, signal.MetricName, namespace, workload)
		if err != nil || baseline == nil {
			continue // Skip signals without baselines
		}

		// Get current value (fallback to baseline mean)
		currentValue, found, _ := s.registry.GetSignalCurrentValue(ctx, signal.MetricName, namespace, workload)
		if !found {
			currentValue = baseline.Mean
		}

		// Compute anomaly score
		score, err := ComputeAnomalyScore(currentValue, *baseline, signal.QualityScore)
		if err != nil {
			continue // Skip signals with insufficient samples
		}

		summaries = append(summaries, SignalSummary{
			MetricName:   signal.MetricName,
			Role:         string(signal.Role),
			Score:        score.Score,
			Confidence:   score.Confidence,
			QualityScore: signal.QualityScore,
		})
	}

	// Sort by score descending, then by confidence descending
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Score != summaries[j].Score {
			return summaries[i].Score > summaries[j].Score
		}
		return summaries[i].Confidence > summaries[j].Confidence
	})

	return &WorkloadSignalsResult{
		Signals: summaries,
		Scope:   fmt.Sprintf("%s/%s", namespace, workload),
	}, nil
}

// GetSignalDetail retrieves detailed baseline and anomaly information for a specific signal.
//
// Process:
// 1. Query registry for specific SignalAnchor
// 2. Fetch baseline and current value
// 3. Compute anomaly score
// 4. Return detailed response
func (s *InvestigateService) GetSignalDetail(
	ctx context.Context,
	namespace, workload, metricName string,
) (*SignalDetailResult, error) {
	if namespace == "" || workload == "" || metricName == "" {
		return nil, fmt.Errorf("namespace, workload, and metric_name are required")
	}

	// Find the signal
	signals, err := s.registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace:    namespace,
		WorkloadName: workload,
	})
	if err != nil {
		return nil, fmt.Errorf("list signals: %w", err)
	}

	var signal *SignalAnchor
	for i := range signals {
		if signals[i].MetricName == metricName {
			signal = &signals[i]
			break
		}
	}

	if signal == nil {
		return nil, fmt.Errorf("signal not found: %s/%s/%s", namespace, workload, metricName)
	}

	// Get baseline
	baseline, err := s.registry.GetSignalBaseline(ctx, metricName, namespace, workload)
	if err != nil {
		return nil, fmt.Errorf("get baseline: %w", err)
	}
	if baseline == nil {
		return nil, fmt.Errorf("signal %s has no baseline (cold start)", metricName)
	}

	// Get current value
	currentValue, found, _ := s.registry.GetSignalCurrentValue(ctx, metricName, namespace, workload)
	if !found {
		currentValue = baseline.Mean
	}

	// Compute anomaly score
	score, err := ComputeAnomalyScore(currentValue, *baseline, signal.QualityScore)
	if err != nil {
		return nil, fmt.Errorf("compute anomaly score: %w", err)
	}

	return &SignalDetailResult{
		MetricName:   metricName,
		Role:         string(signal.Role),
		CurrentValue: currentValue,
		Baseline: BaselineStats{
			Mean:        baseline.Mean,
			StdDev:      baseline.StdDev,
			P50:         baseline.P50,
			P90:         baseline.P90,
			P99:         baseline.P99,
			SampleCount: baseline.SampleCount,
		},
		AnomalyScore:   score.Score,
		Confidence:     score.Confidence,
		SourceProvider: signal.SourceProvider,
		SourceRef:      signal.SourceRef,
		QualityScore:   signal.QualityScore,
	}, nil
}

// CompareSignal compares signal values across time periods.
//
// Note: This requires the provider to support historical value queries.
// Currently returns comparison based on current value vs baseline mean as past proxy.
func (s *InvestigateService) CompareSignal(
	ctx context.Context,
	namespace, workload, metricName string,
	lookback time.Duration,
) (*SignalComparisonResult, error) {
	if namespace == "" || workload == "" || metricName == "" {
		return nil, fmt.Errorf("namespace, workload, and metric_name are required")
	}

	if lookback == 0 {
		lookback = DefaultLookback
	}

	// Get signal detail
	detail, err := s.GetSignalDetail(ctx, namespace, workload, metricName)
	if err != nil {
		return nil, fmt.Errorf("get signal detail: %w", err)
	}

	// Build baseline for scoring
	baseline := SignalBaseline{
		Mean:        detail.Baseline.Mean,
		StdDev:      detail.Baseline.StdDev,
		P50:         detail.Baseline.P50,
		P90:         detail.Baseline.P90,
		P99:         detail.Baseline.P99,
		SampleCount: detail.Baseline.SampleCount,
	}

	currentValue := detail.CurrentValue
	currentScore := detail.AnomalyScore

	// For historical value, we use baseline mean as a proxy
	// In a full implementation, this would query the provider for historical data
	pastValue := baseline.Mean

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
