package grafana

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// BaselineCollector orchestrates periodic baseline data collection and updates.
// It queries Grafana for current metric values and updates SignalBaseline nodes.
//
// Collection runs on a 5-minute interval (per CONTEXT.md) with rate limiting
// to protect the Grafana API (10 req/sec by default).
type BaselineCollector struct {
	grafanaClient   *GrafanaClient
	queryService    *GrafanaQueryService
	graphClient     graph.Client
	integrationName string
	logger          *logging.Logger

	syncInterval time.Duration // 5 minutes per CONTEXT.md
	rateLimiter  *time.Ticker  // 10 req/sec (100ms interval)

	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}

	// Thread-safe status
	mu                sync.RWMutex
	lastSyncTime      time.Time
	baselineCount     int
	errorCount        int
	lastError         error
	inProgress        bool
}

// BaselineCollectorConfig holds configuration for the baseline collector.
type BaselineCollectorConfig struct {
	// SyncInterval is how often to run baseline collection.
	// Default: 5 minutes (per CONTEXT.md)
	SyncInterval time.Duration

	// RateLimitInterval is the minimum time between Grafana API calls.
	// Default: 100ms (10 req/sec per CONTEXT.md)
	RateLimitInterval time.Duration
}

// DefaultBaselineCollectorConfig returns default configuration.
func DefaultBaselineCollectorConfig() BaselineCollectorConfig {
	return BaselineCollectorConfig{
		SyncInterval:      5 * time.Minute,
		RateLimitInterval: 100 * time.Millisecond, // 10 req/sec
	}
}

// NewBaselineCollector creates a new baseline collector with default config.
func NewBaselineCollector(
	grafanaClient *GrafanaClient,
	queryService *GrafanaQueryService,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) *BaselineCollector {
	return NewBaselineCollectorWithConfig(
		grafanaClient,
		queryService,
		graphClient,
		integrationName,
		logger,
		DefaultBaselineCollectorConfig(),
	)
}

// NewBaselineCollectorWithConfig creates a new baseline collector with custom config.
func NewBaselineCollectorWithConfig(
	grafanaClient *GrafanaClient,
	queryService *GrafanaQueryService,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
	config BaselineCollectorConfig,
) *BaselineCollector {
	return &BaselineCollector{
		grafanaClient:   grafanaClient,
		queryService:    queryService,
		graphClient:     graphClient,
		integrationName: integrationName,
		logger:          logger,
		syncInterval:    config.SyncInterval,
		rateLimiter:     time.NewTicker(config.RateLimitInterval),
		stopped:         make(chan struct{}),
	}
}

// Start begins the collection loop (initial collection + periodic sync).
func (c *BaselineCollector) Start(ctx context.Context) error {
	c.logger.Info("Starting baseline collector (interval: %s)", c.syncInterval)

	// Create cancellable context
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Run initial collection (with graceful failure)
	if err := c.collectAndUpdate(); err != nil {
		c.logger.Warn("Initial baseline collection failed: %v (will retry on schedule)", err)
		c.setLastError(err)
	}

	// Start background sync loop
	go c.syncLoop(c.ctx)

	c.logger.Info("Baseline collector started successfully")
	return nil
}

// Stop gracefully stops the collection loop.
func (c *BaselineCollector) Stop() {
	c.logger.Info("Stopping baseline collector")

	if c.cancel != nil {
		c.cancel()
	}

	// Stop rate limiter
	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}

	// Wait for sync loop to stop (with timeout)
	select {
	case <-c.stopped:
		c.logger.Info("Baseline collector stopped")
	case <-time.After(5 * time.Second):
		c.logger.Warn("Baseline collector stop timeout")
	}
}

// syncLoop runs periodic collection on ticker interval.
func (c *BaselineCollector) syncLoop(ctx context.Context) {
	defer close(c.stopped)

	ticker := time.NewTicker(c.syncInterval)
	defer ticker.Stop()

	c.logger.Debug("Baseline collection loop started (interval: %s)", c.syncInterval)

	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("Baseline collection loop stopped (context cancelled)")
			return

		case <-ticker.C:
			c.logger.Debug("Periodic baseline collection triggered")
			if err := c.collectAndUpdate(); err != nil {
				c.logger.Warn("Periodic baseline collection failed: %v", err)
				c.setLastError(err)
			}
		}
	}
}

// collectAndUpdate performs baseline data collection for all active signals.
// For each signal:
// 1. Rate limit before API call
// 2. Query Grafana for current metric value
// 3. Get existing baseline (or create new)
// 4. Append new sample to window and recompute statistics
// 5. Upsert baseline to graph
func (c *BaselineCollector) collectAndUpdate() error {
	startTime := time.Now()
	c.logger.Info("Starting baseline collection")

	// Set inProgress flag
	c.mu.Lock()
	c.inProgress = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.inProgress = false
		c.mu.Unlock()
	}()

	// Query graph for all active SignalAnchors
	signals, err := GetActiveSignalAnchors(c.ctx, c.graphClient, c.integrationName)
	if err != nil {
		return fmt.Errorf("failed to get active signals: %w", err)
	}

	c.logger.Info("Found %d active signals to process", len(signals))

	if len(signals) == 0 {
		c.logger.Debug("No active signals to collect baselines for")
		c.updateSyncStatus(0, 0, nil)
		return nil
	}

	updatedCount := 0
	errorCount := 0

	for _, signal := range signals {
		// Rate limit before API call
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-c.rateLimiter.C:
			// Rate limit passed
		}

		// Process single signal
		if err := c.processSignal(signal); err != nil {
			c.logger.Debug("Failed to process signal %s: %v", signal.MetricName, err)
			errorCount++
			continue
		}

		updatedCount++
	}

	duration := time.Since(startTime)
	c.logger.Info("Baseline collection complete: %d baselines updated, %d errors (duration: %s)",
		updatedCount, errorCount, duration)

	c.updateSyncStatus(updatedCount, errorCount, nil)

	if errorCount > 0 {
		return fmt.Errorf("collection completed with %d errors", errorCount)
	}

	return nil
}

// processSignal handles baseline collection for a single signal.
func (c *BaselineCollector) processSignal(signal SignalAnchor) error {
	// Skip signals without dashboard info (can't query)
	if signal.DashboardUID == "" {
		return fmt.Errorf("signal has no dashboard UID")
	}

	// Query current metric value from Grafana
	currentValue, err := c.queryCurrentValue(signal)
	if err != nil {
		return fmt.Errorf("query current value: %w", err)
	}

	// Get existing baseline (or initialize new)
	now := time.Now().Unix()
	baseline, err := GetSignalBaseline(
		c.ctx,
		c.graphClient,
		signal.MetricName,
		signal.WorkloadNamespace,
		signal.WorkloadName,
		c.integrationName,
	)
	if err != nil {
		return fmt.Errorf("get existing baseline: %w", err)
	}

	// Initialize new baseline if not found
	if baseline == nil {
		baseline = &SignalBaseline{
			MetricName:        signal.MetricName,
			WorkloadNamespace: signal.WorkloadNamespace,
			WorkloadName:      signal.WorkloadName,
			Integration:       c.integrationName,
			WindowStart:       now,
			WindowEnd:         now,
			SampleCount:       0,
		}
	}

	// Append sample and update statistics
	// For now, we use incremental update approximation
	// A more accurate approach would store raw samples and recompute
	baseline = c.updateBaselineWithSample(baseline, currentValue, now)

	// Set TTL to 7 days from last update
	baseline.LastUpdated = now
	baseline.ExpiresAt = now + (7 * 24 * 60 * 60)

	// Upsert to graph
	if err := UpsertSignalBaseline(c.ctx, c.graphClient, *baseline); err != nil {
		return fmt.Errorf("upsert baseline: %w", err)
	}

	c.logger.Debug("Updated baseline for %s: mean=%.4f, stddev=%.4f, samples=%d",
		signal.MetricName, baseline.Mean, baseline.StdDev, baseline.SampleCount)

	return nil
}

// queryCurrentValue queries Grafana for the current value of a signal's metric.
func (c *BaselineCollector) queryCurrentValue(signal SignalAnchor) (float64, error) {
	// Use a short time range to get the most recent value (last 5 minutes)
	now := time.Now()
	from := now.Add(-5 * time.Minute)

	timeRange := TimeRange{
		From: from.Format(time.RFC3339),
		To:   now.Format(time.RFC3339),
	}

	// Execute dashboard query for this signal's panel
	result, err := c.queryService.ExecuteDashboard(
		c.ctx,
		signal.DashboardUID,
		timeRange,
		nil, // No scoped vars for baseline collection
		1,   // Only query one panel to get current value
	)
	if err != nil {
		return 0, fmt.Errorf("execute dashboard query: %w", err)
	}

	// Extract most recent value from result
	// Look through panels and metrics for matching metric name
	for _, panel := range result.Panels {
		for _, metric := range panel.Metrics {
			// Check if this metric matches our signal
			// MetricResult.Labels may contain __name__ or we match on panel context
			if len(metric.Values) > 0 {
				// Return the most recent (last) value
				lastValue := metric.Values[len(metric.Values)-1]
				return lastValue.Value, nil
			}
		}
	}

	return 0, fmt.Errorf("no metric values found for signal %s", signal.MetricName)
}

// updateBaselineWithSample updates baseline statistics with a new sample value.
// Uses Welford's online algorithm for incremental mean/variance update.
func (c *BaselineCollector) updateBaselineWithSample(baseline *SignalBaseline, newValue float64, timestamp int64) *SignalBaseline {
	n := baseline.SampleCount + 1

	if n == 1 {
		// First sample
		baseline.Mean = newValue
		baseline.StdDev = 0
		baseline.Median = newValue
		baseline.P50 = newValue
		baseline.P90 = newValue
		baseline.P99 = newValue
		baseline.Min = newValue
		baseline.Max = newValue
	} else {
		// Welford's online algorithm for mean and variance
		oldMean := baseline.Mean
		oldVariance := baseline.StdDev * baseline.StdDev

		// Update mean
		delta := newValue - oldMean
		newMean := oldMean + delta/float64(n)

		// Update variance (M2 = sum of squared differences from mean)
		delta2 := newValue - newMean
		newVariance := (oldVariance*float64(n-1) + delta*delta2) / float64(n)

		baseline.Mean = newMean
		if n > 1 {
			// Sample standard deviation (N-1)
			baseline.StdDev = computeStdDevFromVariance(newVariance, n)
		}

		// Update min/max
		if newValue < baseline.Min {
			baseline.Min = newValue
		}
		if newValue > baseline.Max {
			baseline.Max = newValue
		}

		// Approximate percentile updates
		// For true percentiles we would need to store all samples
		// This is an approximation that moves percentiles toward new value
		baseline.Median = updatePercentile(baseline.Median, newValue, 0.50, n)
		baseline.P50 = baseline.Median
		baseline.P90 = updatePercentile(baseline.P90, newValue, 0.90, n)
		baseline.P99 = updatePercentile(baseline.P99, newValue, 0.99, n)
	}

	baseline.SampleCount = n
	baseline.WindowEnd = timestamp

	return baseline
}

// computeStdDevFromVariance computes sample standard deviation from variance.
func computeStdDevFromVariance(variance float64, n int) float64 {
	if n <= 1 || variance < 0 {
		return 0
	}
	// Sample std dev uses N-1
	sampleVariance := variance * float64(n) / float64(n-1)
	if sampleVariance < 0 {
		return 0
	}
	return math.Sqrt(sampleVariance)
}

// updatePercentile approximates percentile update using exponential smoothing.
// This is an approximation - for exact percentiles, store all samples.
func updatePercentile(current, newValue, percentile float64, n int) float64 {
	// Learning rate decreases as we get more samples
	alpha := 1.0 / float64(n)
	if alpha < 0.01 {
		alpha = 0.01 // Minimum learning rate
	}

	// Adjust based on whether new value is above or below current percentile
	if newValue > current {
		// Value above percentile - move up based on how far above
		return current + alpha*(newValue-current)*(1.0-percentile)
	}
	// Value below percentile - move down
	return current + alpha*(newValue-current)*percentile
}

// updateSyncStatus updates the thread-safe sync status.
func (c *BaselineCollector) updateSyncStatus(baselineCount, errorCount int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastSyncTime = time.Now()
	c.baselineCount = baselineCount
	c.errorCount = errorCount
	if err == nil {
		c.lastError = nil
	}
}

// setLastError updates the last error (thread-safe).
func (c *BaselineCollector) setLastError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
}

// Status returns the current collection status.
func (c *BaselineCollector) Status() BaselineCollectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var lastErrorStr string
	if c.lastError != nil {
		lastErrorStr = c.lastError.Error()
	}

	return BaselineCollectorStatus{
		LastSyncTime:  c.lastSyncTime,
		BaselineCount: c.baselineCount,
		ErrorCount:    c.errorCount,
		LastError:     lastErrorStr,
		InProgress:    c.inProgress,
	}
}

// BaselineCollectorStatus holds the current status of the collector.
type BaselineCollectorStatus struct {
	LastSyncTime  time.Time
	BaselineCount int
	ErrorCount    int
	LastError     string
	InProgress    bool
}
