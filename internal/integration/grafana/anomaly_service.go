package grafana

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// AnomalyService orchestrates anomaly detection flow:
// - Fetches current metrics
// - Computes/retrieves baselines from 7-day history
// - Detects anomalies via statistical detector
// - Ranks and limits results
type AnomalyService struct {
	queryService  *GrafanaQueryService
	detector      *StatisticalDetector
	baselineCache *BaselineCache
	logger        *logging.Logger
}

// NewAnomalyService creates a new anomaly service instance
func NewAnomalyService(
	queryService *GrafanaQueryService,
	detector *StatisticalDetector,
	baselineCache *BaselineCache,
	logger *logging.Logger,
) *AnomalyService {
	return &AnomalyService{
		queryService:  queryService,
		detector:      detector,
		baselineCache: baselineCache,
		logger:        logger,
	}
}

// AnomalyResult represents the result of anomaly detection
type AnomalyResult struct {
	Anomalies      []MetricAnomaly `json:"anomalies"`
	MetricsChecked int             `json:"metrics_checked"`
	TimeRange      string          `json:"time_range"`
	SkipCount      int             `json:"metrics_skipped"`
}

// HistoricalDataPoint represents a single time-series data point from historical data.
// Extracted from Grafana DataFrame.Data.Values where Values[0] is timestamps
// and Values[1] is metric values.
type HistoricalDataPoint struct {
	Timestamp time.Time
	Value     float64
}

// DetectAnomalies performs anomaly detection on metrics from a dashboard
// Returns top 20 anomalies ranked by severity (critical > warning > info) then z-score
func (s *AnomalyService) DetectAnomalies(
	ctx context.Context,
	dashboardUID string,
	timeRange TimeRange,
	scopedVars map[string]string,
) (*AnomalyResult, error) {
	// Parse current time from timeRange.To
	currentTime, err := time.Parse(time.RFC3339, timeRange.To)
	if err != nil {
		return nil, fmt.Errorf("parse time range to: %w", err)
	}

	// Fetch current metrics (maxPanels=5 for overview)
	dashboardResult, err := s.queryService.ExecuteDashboard(ctx, dashboardUID, timeRange, scopedVars, 5)
	if err != nil {
		return nil, fmt.Errorf("fetch current metrics: %w", err)
	}

	anomalies := make([]MetricAnomaly, 0)
	skipCount := 0
	metricsChecked := 0

	// Process each panel result
	for _, panelResult := range dashboardResult.Panels {
		for _, series := range panelResult.Metrics {
			metricsChecked++

			// Extract metric name from labels (use __name__ label or construct from all labels)
			metricName := extractMetricName(series.Labels)
			if metricName == "" {
				s.logger.Debug("Skipping metric with no name in panel %d", panelResult.PanelID)
				skipCount++
				continue
			}

			// Get most recent value (last in series)
			if len(series.Values) == 0 {
				s.logger.Debug("Skipping metric %s with no values", metricName)
				skipCount++
				continue
			}
			currentValue := series.Values[len(series.Values)-1].Value

			// Check baseline cache
			baseline, err := s.baselineCache.Get(ctx, metricName, currentTime)
			if err != nil {
				s.logger.Warn("Failed to get baseline from cache for %s: %v", metricName, err)
				skipCount++
				continue
			}

			// Cache miss - compute baseline from 7-day history
			if baseline == nil {
				baseline, err = s.computeBaseline(ctx, dashboardUID, metricName, currentTime, scopedVars)
				if err != nil {
					s.logger.Warn("Failed to compute baseline for %s: %v", metricName, err)
					skipCount++
					continue
				}

				// Baseline computation returned nil (insufficient data) - skip metric silently
				if baseline == nil {
					s.logger.Debug("Insufficient historical data for %s, skipping", metricName)
					skipCount++
					continue
				}

				// Store in cache with 1-hour TTL
				if err := s.baselineCache.Set(ctx, baseline, time.Hour); err != nil {
					s.logger.Warn("Failed to cache baseline for %s: %v", metricName, err)
					// Continue with detection despite cache failure
				}
			}

			// Detect anomaly
			anomaly := s.detector.Detect(metricName, currentValue, *baseline, currentTime)
			if anomaly != nil {
				anomalies = append(anomalies, *anomaly)
			}
		}
	}

	// Rank anomalies: sort by severity (critical > warning > info), then z-score descending
	sort.Slice(anomalies, func(i, j int) bool {
		// Define severity rank
		severityRank := map[string]int{
			"critical": 3,
			"warning":  2,
			"info":     1,
		}

		rankI := severityRank[anomalies[i].Severity]
		rankJ := severityRank[anomalies[j].Severity]

		if rankI != rankJ {
			return rankI > rankJ // Higher rank first (critical > warning > info)
		}

		// Same severity - sort by absolute z-score descending
		absZI := anomalies[i].ZScore
		if absZI < 0 {
			absZI = -absZI
		}
		absZJ := anomalies[j].ZScore
		if absZJ < 0 {
			absZJ = -absZJ
		}
		return absZI > absZJ
	})

	// Limit to top 20 anomalies
	if len(anomalies) > 20 {
		anomalies = anomalies[:20]
	}

	return &AnomalyResult{
		Anomalies:      anomalies,
		MetricsChecked: metricsChecked,
		TimeRange:      timeRange.FormatDisplay(),
		SkipCount:      skipCount,
	}, nil
}

// computeBaseline computes baseline from 7-day historical data with time-of-day matching
// Returns nil if insufficient samples (< 3 matching windows)
func (s *AnomalyService) computeBaseline(
	ctx context.Context,
	dashboardUID string,
	metricName string,
	currentTime time.Time,
	scopedVars map[string]string,
) (*Baseline, error) {
	// Compute 7-day historical time range ending at currentTime
	historicalFrom := currentTime.Add(-7 * 24 * time.Hour)
	historicalTimeRange := TimeRange{
		From: historicalFrom.Format(time.RFC3339),
		To:   currentTime.Format(time.RFC3339),
	}

	s.logger.Debug("Computing baseline for %s from %s to %s",
		metricName, historicalTimeRange.From, historicalTimeRange.To)

	// Query historical data via ExecuteDashboard
	// Note: This fetches ALL panels - we'll filter to matching metric later
	dashboardResult, err := s.queryService.ExecuteDashboard(
		ctx, dashboardUID, historicalTimeRange, scopedVars, 0, // maxPanels=0 for all
	)
	if err != nil {
		return nil, fmt.Errorf("fetch historical data: %w", err)
	}

	// Extract time-series data for the target metric
	historicalData := make([]HistoricalDataPoint, 0)
	for _, panelResult := range dashboardResult.Panels {
		for _, series := range panelResult.Metrics {
			// Check if this series matches our target metric
			seriesMetricName := extractMetricName(series.Labels)
			if seriesMetricName != metricName {
				continue
			}

			// Parse time-series data from DataFrame (already parsed in series.Values)
			for _, dataPoint := range series.Values {
				timestamp, err := time.Parse(time.RFC3339, dataPoint.Timestamp)
				if err != nil {
					s.logger.Debug("Failed to parse timestamp %s: %v", dataPoint.Timestamp, err)
					continue
				}

				historicalData = append(historicalData, HistoricalDataPoint{
					Timestamp: timestamp,
					Value:     dataPoint.Value,
				})
			}
		}
	}

	if len(historicalData) == 0 {
		s.logger.Debug("No historical data found for %s", metricName)
		return nil, nil // Insufficient data - return nil to trigger silent skip
	}

	// Apply time-of-day matching
	matchedValues := matchTimeWindows(currentTime, historicalData)

	// Require minimum 3 matching windows
	if len(matchedValues) < 3 {
		s.logger.Debug("Insufficient matching windows for %s: got %d, need 3",
			metricName, len(matchedValues))
		return nil, nil // Insufficient data - return nil to trigger silent skip
	}

	// Compute mean and standard deviation
	mean := computeMean(matchedValues)
	stddev := computeStdDev(matchedValues, mean)

	baseline := &Baseline{
		MetricName:  metricName,
		Mean:        mean,
		StdDev:      stddev,
		SampleCount: len(matchedValues),
		WindowHour:  currentTime.Hour(),
		DayType:     getDayType(currentTime),
	}

	s.logger.Debug("Computed baseline for %s: mean=%.2f, stddev=%.2f, samples=%d",
		metricName, mean, stddev, len(matchedValues))

	return baseline, nil
}

// matchTimeWindows filters historical data to matching hour and day type
// Returns matched values for baseline computation
func matchTimeWindows(currentTime time.Time, historicalData []HistoricalDataPoint) []float64 {
	targetHour := currentTime.Hour()
	targetDayType := getDayType(currentTime)

	matchedValues := make([]float64, 0)
	for _, point := range historicalData {
		if point.Timestamp.Hour() == targetHour && getDayType(point.Timestamp) == targetDayType {
			matchedValues = append(matchedValues, point.Value)
		}
	}

	return matchedValues
}

// extractMetricName extracts a metric name from labels
// Prefers __name__ label, falls back to constructing from all labels
func extractMetricName(labels map[string]string) string {
	// Try __name__ label first (standard Prometheus metric name)
	if name, ok := labels["__name__"]; ok && name != "" {
		return name
	}

	// If no __name__, construct a name from all labels for identification
	// This handles cases where labels don't include __name__
	if len(labels) == 0 {
		return ""
	}

	// Simple fallback: use first label value as identifier
	for k, v := range labels {
		if v != "" {
			return fmt.Sprintf("%s=%s", k, v)
		}
	}

	return ""
}
