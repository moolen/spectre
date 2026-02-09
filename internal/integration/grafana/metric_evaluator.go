package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// MetricWindow holds metric values for a time window
type MetricWindow struct {
	Start      time.Time
	End        time.Time
	Values     []float64
	Timestamps []time.Time
}

// MetricEvaluator queries Prometheus via Grafana for metric values around alert transitions.
type MetricEvaluator struct {
	grafanaClient  GrafanaClientInterface
	datasourceUID  string
	windowSize     time.Duration
	minSampleCount int
	queryRateLimit time.Duration
	lastQueryTime  time.Time
	logger         *logging.Logger
}

// NewMetricEvaluator creates a new MetricEvaluator.
func NewMetricEvaluator(
	grafanaClient GrafanaClientInterface,
	datasourceUID string,
	windowSize time.Duration,
	minSampleCount int,
	queryRateLimit time.Duration,
	logger *logging.Logger,
) *MetricEvaluator {
	return &MetricEvaluator{
		grafanaClient:  grafanaClient,
		datasourceUID:  datasourceUID,
		windowSize:     windowSize,
		minSampleCount: minSampleCount,
		queryRateLimit: queryRateLimit,
		logger:         logger,
	}
}

// GetMetricWindows queries metric values for before/after windows around a transition.
//
// Time windows:
//
//	Before: [transition - forDuration - windowSize, transition - forDuration]
//	After:  [transition, transition + windowSize]
//
// Parameters:
//
//	metricName: The metric to query (e.g., "container_cpu_usage_seconds_total")
//	namespace:  Namespace filter for the query
//	transition: The alert transition timestamp
//	forDuration: The alert's `for:` duration (to offset the before window)
//
// Returns before and after windows, or error if insufficient data.
func (e *MetricEvaluator) GetMetricWindows(
	ctx context.Context,
	metricName string,
	namespace string,
	transitionTime time.Time,
	forDuration time.Duration,
) (before *MetricWindow, after *MetricWindow, err error) {
	// Apply rate limiting
	e.rateLimitWait()

	// Calculate time windows
	// Before window ends at transition time minus forDuration (when alert started evaluating)
	beforeEnd := transitionTime.Add(-forDuration)
	beforeStart := beforeEnd.Add(-e.windowSize)

	// After window starts at transition time
	afterStart := transitionTime
	afterEnd := afterStart.Add(e.windowSize)

	// Build PromQL query
	promQL := e.buildPromQLQuery(metricName, namespace)

	e.logger.Debug("Querying metric %s for windows: before=[%s, %s], after=[%s, %s]",
		metricName, beforeStart.Format(time.RFC3339), beforeEnd.Format(time.RFC3339),
		afterStart.Format(time.RFC3339), afterEnd.Format(time.RFC3339))

	// Query before window
	beforeValues, beforeTimestamps, err := e.queryMetricRange(ctx, promQL, beforeStart, beforeEnd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query before window: %w", err)
	}

	// Query after window
	afterValues, afterTimestamps, err := e.queryMetricRange(ctx, promQL, afterStart, afterEnd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query after window: %w", err)
	}

	// Check minimum sample counts
	if len(beforeValues) < e.minSampleCount {
		return nil, nil, fmt.Errorf("insufficient samples in before window: got %d, need %d",
			len(beforeValues), e.minSampleCount)
	}
	if len(afterValues) < e.minSampleCount {
		return nil, nil, fmt.Errorf("insufficient samples in after window: got %d, need %d",
			len(afterValues), e.minSampleCount)
	}

	before = &MetricWindow{
		Start:      beforeStart,
		End:        beforeEnd,
		Values:     beforeValues,
		Timestamps: beforeTimestamps,
	}

	after = &MetricWindow{
		Start:      afterStart,
		End:        afterEnd,
		Values:     afterValues,
		Timestamps: afterTimestamps,
	}

	return before, after, nil
}

// buildPromQLQuery constructs the PromQL query with namespace filter.
func (e *MetricEvaluator) buildPromQLQuery(metricName, namespace string) string {
	if namespace != "" {
		return fmt.Sprintf(`%s{namespace="%s"}`, metricName, namespace)
	}
	return metricName
}

// queryMetricRange queries Prometheus via Grafana for metric values in a time range.
func (e *MetricEvaluator) queryMetricRange(
	ctx context.Context,
	promQL string,
	start, end time.Time,
) ([]float64, []time.Time, error) {
	// Format times for Grafana API
	fromStr := fmt.Sprintf("%d", start.UnixMilli())
	toStr := fmt.Sprintf("%d", end.UnixMilli())

	// Query via Grafana
	response, err := e.grafanaClient.QueryDataSource(ctx, e.datasourceUID, promQL, fromStr, toStr, nil)
	if err != nil {
		return nil, nil, err
	}

	var values []float64
	var timestamps []time.Time

	// Extract values from response frames
	for _, queryResult := range response.Results {
		if queryResult.Error != "" {
			continue
		}
		for _, frame := range queryResult.Frames {
			if len(frame.Data.Values) >= 2 {
				// Values[0] = timestamps, Values[1] = values
				tsValues := frame.Data.Values[0]
				metricValues := frame.Data.Values[1]

				for i := range metricValues {
					if i < len(tsValues) {
						// Try to parse timestamp
						var ts time.Time
						switch v := tsValues[i].(type) {
						case float64:
							ts = time.UnixMilli(int64(v))
						case int64:
							ts = time.UnixMilli(v)
						}

						// Parse metric value
						var val float64
						switch v := metricValues[i].(type) {
						case float64:
							val = v
						case int64:
							val = float64(v)
						case int:
							val = float64(v)
						}

						if !ts.IsZero() {
							timestamps = append(timestamps, ts)
							values = append(values, val)
						}
					}
				}
			}
		}
	}

	return values, timestamps, nil
}

// rateLimitWait waits if necessary to respect the query rate limit.
func (e *MetricEvaluator) rateLimitWait() {
	if e.queryRateLimit <= 0 {
		return
	}

	elapsed := time.Since(e.lastQueryTime)
	if elapsed < e.queryRateLimit {
		time.Sleep(e.queryRateLimit - elapsed)
	}
	e.lastQueryTime = time.Now()
}
