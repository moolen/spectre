package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// BaselineCache provides caching for computed baselines using FalkorDB graph storage
type BaselineCache struct {
	graphClient graph.Client
	logger      *logging.Logger
}

// NewBaselineCache creates a new baseline cache instance
func NewBaselineCache(graphClient graph.Client, logger *logging.Logger) *BaselineCache {
	return &BaselineCache{
		graphClient: graphClient,
		logger:      logger,
	}
}

// Get retrieves a cached baseline for the given metric and time context
// Returns nil if no valid cached baseline exists (cache miss)
func (bc *BaselineCache) Get(ctx context.Context, metricName string, t time.Time) (*Baseline, error) {
	hour := t.Hour()
	dayType := getDayType(t)
	now := time.Now().Unix()

	bc.logger.Debug("Cache lookup: metric=%s, hour=%d, day_type=%s", metricName, hour, dayType)

	// Query FalkorDB for matching baseline node with TTL filtering
	query := `
		MATCH (b:Baseline {
			metric_name: $metric_name,
			window_hour: $window_hour,
			day_type: $day_type
		})
		WHERE b.expires_at > $now
		RETURN b.mean, b.stddev, b.sample_count
	`

	result, err := bc.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name": metricName,
			"window_hour": hour,
			"day_type":    dayType,
			"now":         now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query baseline cache: %w", err)
	}

	// Cache miss if no rows returned
	if len(result.Rows) == 0 {
		bc.logger.Debug("Cache miss: metric=%s, hour=%d, day_type=%s", metricName, hour, dayType)
		return nil, nil
	}

	// Parse result into Baseline struct
	row := result.Rows[0]
	if len(row) < 3 {
		return nil, fmt.Errorf("invalid result row: expected 3 columns, got %d", len(row))
	}

	// Extract values with type assertions
	mean, err := toFloat64(row[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse mean: %w", err)
	}

	stddev, err := toFloat64(row[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse stddev: %w", err)
	}

	sampleCount, err := toInt(row[2])
	if err != nil {
		return nil, fmt.Errorf("failed to parse sample_count: %w", err)
	}

	baseline := &Baseline{
		MetricName:  metricName,
		Mean:        mean,
		StdDev:      stddev,
		SampleCount: sampleCount,
		WindowHour:  hour,
		DayType:     dayType,
	}

	bc.logger.Debug("Cache hit: metric=%s, hour=%d, day_type=%s, mean=%.2f, stddev=%.2f",
		metricName, hour, dayType, mean, stddev)

	return baseline, nil
}

// Set stores a baseline in the cache with the specified TTL
func (bc *BaselineCache) Set(ctx context.Context, baseline *Baseline, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl).Unix()

	bc.logger.Debug("Cache write: metric=%s, hour=%d, day_type=%s, ttl=%v",
		baseline.MetricName, baseline.WindowHour, baseline.DayType, ttl)

	// Use MERGE for upsert semantics (create or update)
	query := `
		MERGE (b:Baseline {
			metric_name: $metric_name,
			window_hour: $window_hour,
			day_type: $day_type
		})
		SET b.mean = $mean,
		    b.stddev = $stddev,
		    b.sample_count = $sample_count,
		    b.expires_at = $expires_at
	`

	_, err := bc.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name":  baseline.MetricName,
			"window_hour":  baseline.WindowHour,
			"day_type":     baseline.DayType,
			"mean":         baseline.Mean,
			"stddev":       baseline.StdDev,
			"sample_count": baseline.SampleCount,
			"expires_at":   expiresAt,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to write baseline cache: %w", err)
	}

	bc.logger.Debug("Cache write successful: metric=%s, expires_at=%d", baseline.MetricName, expiresAt)
	return nil
}

// getDayType returns "weekend" for Saturday/Sunday, "weekday" otherwise
func getDayType(t time.Time) string {
	if isWeekend(t) {
		return "weekend"
	}
	return "weekday"
}

// isWeekend checks if the given time falls on Saturday or Sunday
func isWeekend(t time.Time) bool {
	weekday := t.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// toFloat64 converts interface{} to float64, handling both int64 and float64 from FalkorDB
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int64:
		return float64(val), nil
	case int:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// toInt converts interface{} to int, handling both int64 and float64 from FalkorDB
func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case int:
		return val, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}
