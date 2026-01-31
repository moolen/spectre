package grafana

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// SignalValidationJobStatus holds the current status of the job
type SignalValidationJobStatus struct {
	LastRunTime          time.Time     `json:"lastRunTime"`
	LastRunDuration      time.Duration `json:"lastRunDuration"`
	AlertsProcessed      int           `json:"alertsProcessed"`
	TransitionsEvaluated int           `json:"transitionsEvaluated"`
	CorrelationsFound    int           `json:"correlationsFound"`
	CorrelationsUpdated  int           `json:"correlationsUpdated"`
	Errors               int           `json:"errors"`
	InProgress           bool          `json:"inProgress"`
	LastError            string        `json:"lastError"`
	NextScheduledRun     time.Time     `json:"nextScheduledRun"`
}

// SignalValidationJob orchestrates the correlation analysis between
// alert state transitions and signal behavior.
type SignalValidationJob struct {
	// Dependencies
	grafanaClient   GrafanaClientInterface
	graphClient     graph.Client
	integrationName string
	config          SignalValidationConfig
	logger          *logging.Logger

	// Components
	flappingDetector    *FlappingDetector
	alertSignalMatcher  *AlertSignalMatcher
	metricEvaluator     *MetricEvaluator
	statisticalAnalyzer *StatisticalAnalyzer
	correlationStore    *CorrelationStore

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}

	// Status
	mu     sync.RWMutex
	status SignalValidationJobStatus
}

// NewSignalValidationJob creates a new SignalValidationJob.
func NewSignalValidationJob(
	grafanaClient GrafanaClientInterface,
	graphClient graph.Client,
	integrationName string,
	datasourceUID string,
	config SignalValidationConfig,
	logger *logging.Logger,
) *SignalValidationJob {
	cfg := config.WithDefaults()

	return &SignalValidationJob{
		grafanaClient:   grafanaClient,
		graphClient:     graphClient,
		integrationName: integrationName,
		config:          cfg,
		logger:          logger,
		stopped:         make(chan struct{}),

		// Initialize components
		flappingDetector: NewFlappingDetector(
			cfg.FlappingMaxTransitionsPerDay,
			cfg.GetFlappingMaxDuration(),
		),
		alertSignalMatcher: NewAlertSignalMatcher(graphClient, integrationName, logger),
		metricEvaluator: NewMetricEvaluator(
			grafanaClient,
			datasourceUID,
			cfg.GetWindowSize(),
			cfg.MinSampleCount,
			cfg.GetQueryRateLimit(),
			logger,
		),
		statisticalAnalyzer: NewStatisticalAnalyzer(
			cfg.PValueThreshold,
			cfg.CohensDThreshold,
			cfg.SigmaThreshold,
		),
		correlationStore: NewCorrelationStore(
			graphClient,
			integrationName,
			cfg.GetDecayPeriod(),
			logger,
		),
	}
}

// Start begins the background job with periodic execution.
func (j *SignalValidationJob) Start(ctx context.Context) error {
	j.logger.Info("Starting signal validation job (interval: %s)", j.config.GetRunInterval())

	j.ctx, j.cancel = context.WithCancel(ctx)

	// Start background sync loop
	go j.syncLoop(j.ctx)

	j.logger.Info("Signal validation job started successfully")
	return nil
}

// Stop gracefully stops the job.
func (j *SignalValidationJob) Stop() {
	j.logger.Info("Stopping signal validation job")

	if j.cancel != nil {
		j.cancel()
	}

	select {
	case <-j.stopped:
		j.logger.Info("Signal validation job stopped")
	case <-time.After(30 * time.Second):
		j.logger.Warn("Signal validation job stop timeout")
	}
}

// RunNow triggers an immediate incremental run (last 24h transitions).
func (j *SignalValidationJob) RunNow(ctx context.Context) error {
	return j.run(ctx, false)
}

// RunFull triggers a full backfill run (all transitions within lookback period).
func (j *SignalValidationJob) RunFull(ctx context.Context) error {
	return j.run(ctx, true)
}

// RunForAlert runs validation for a specific alert with full lookback.
func (j *SignalValidationJob) RunForAlert(ctx context.Context, alertUID string) error {
	j.logger.Info("Running signal validation for alert %s", alertUID)

	j.mu.Lock()
	if j.status.InProgress {
		j.mu.Unlock()
		return fmt.Errorf("job already in progress")
	}
	j.status.InProgress = true
	j.mu.Unlock()

	defer func() {
		j.mu.Lock()
		j.status.InProgress = false
		j.mu.Unlock()
	}()

	// Get alert's PromQL
	promQL, title, err := j.alertSignalMatcher.GetAlertPromQL(ctx, alertUID)
	if err != nil {
		return fmt.Errorf("failed to get alert PromQL: %w", err)
	}

	// Fetch transitions for this alert
	endTime := time.Now()
	startTime := endTime.Add(-j.config.GetLookbackPeriod())

	transitions, err := FetchStateTransitions(ctx, j.graphClient, alertUID, j.integrationName, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to fetch transitions: %w", err)
	}

	if len(transitions) == 0 {
		j.logger.Debug("No transitions found for alert %s", alertUID)
		return nil
	}

	// Check for flapping
	if j.flappingDetector.IsFlapping(transitions) {
		j.logger.Debug("Alert %s is flapping, skipping", alertUID)
		return nil
	}

	// Process the alert
	stats, err := j.processAlert(ctx, alertUID, title, promQL, transitions)
	if err != nil {
		return err
	}

	j.mu.Lock()
	j.status.AlertsProcessed++
	j.status.TransitionsEvaluated += stats.transitionsEvaluated
	j.status.CorrelationsFound += stats.correlationsFound
	j.status.CorrelationsUpdated += stats.correlationsUpdated
	j.mu.Unlock()

	return nil
}

// Status returns the current job status.
func (j *SignalValidationJob) Status() SignalValidationJobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

// syncLoop runs periodic validation on ticker interval.
func (j *SignalValidationJob) syncLoop(ctx context.Context) {
	defer close(j.stopped)

	ticker := time.NewTicker(j.config.GetRunInterval())
	defer ticker.Stop()

	// Update next scheduled run
	j.mu.Lock()
	j.status.NextScheduledRun = time.Now().Add(j.config.GetRunInterval())
	j.mu.Unlock()

	j.logger.Debug("Signal validation sync loop started (interval: %s)", j.config.GetRunInterval())

	for {
		select {
		case <-ctx.Done():
			j.logger.Debug("Signal validation sync loop stopped (context cancelled)")
			return

		case <-ticker.C:
			j.logger.Debug("Periodic signal validation triggered")
			if err := j.run(ctx, false); err != nil {
				j.logger.Warn("Periodic signal validation failed: %v", err)
				j.setLastError(err)
			}

			// Update next scheduled run
			j.mu.Lock()
			j.status.NextScheduledRun = time.Now().Add(j.config.GetRunInterval())
			j.mu.Unlock()
		}
	}
}

// run executes one validation pass.
func (j *SignalValidationJob) run(ctx context.Context, fullRun bool) error {
	startTime := time.Now()
	j.logger.Info("Starting signal validation run (full=%v)", fullRun)

	j.mu.Lock()
	if j.status.InProgress {
		j.mu.Unlock()
		return fmt.Errorf("job already in progress")
	}
	j.status.InProgress = true
	j.status.AlertsProcessed = 0
	j.status.TransitionsEvaluated = 0
	j.status.CorrelationsFound = 0
	j.status.CorrelationsUpdated = 0
	j.status.Errors = 0
	j.mu.Unlock()

	defer func() {
		j.mu.Lock()
		j.status.InProgress = false
		j.status.LastRunTime = startTime
		j.status.LastRunDuration = time.Since(startTime)
		j.mu.Unlock()
	}()

	// First, reconcile new alerts (process any alerts with transitions but no correlations)
	newAlerts, err := j.correlationStore.ListUncorrelatedAlerts(ctx, 100)
	if err != nil {
		j.logger.Warn("Failed to list uncorrelated alerts: %v", err)
	} else {
		j.logger.Debug("Found %d new alerts to process", len(newAlerts))
		for _, alertUID := range newAlerts {
			if err := j.processAlertByUID(ctx, alertUID, true); err != nil {
				j.logger.Warn("Failed to process new alert %s: %v", alertUID, err)
				j.mu.Lock()
				j.status.Errors++
				j.mu.Unlock()
			}
		}
	}

	// Get all alerts with transitions
	alertUIDs, err := j.alertSignalMatcher.ListAlertsWithTransitions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list alerts: %w", err)
	}

	j.logger.Info("Found %d alerts with transitions to process", len(alertUIDs))

	// Process each alert
	for _, alertUID := range alertUIDs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := j.processAlertByUID(ctx, alertUID, fullRun); err != nil {
			j.logger.Debug("Failed to process alert %s: %v", alertUID, err)
			j.mu.Lock()
			j.status.Errors++
			j.mu.Unlock()
			continue
		}
	}

	j.mu.RLock()
	stats := j.status
	j.mu.RUnlock()

	j.logger.Info("Signal validation run complete: %d alerts, %d transitions, %d correlations found, %d errors (duration: %s)",
		stats.AlertsProcessed, stats.TransitionsEvaluated, stats.CorrelationsFound, stats.Errors, time.Since(startTime))

	if stats.Errors > 0 {
		return fmt.Errorf("completed with %d errors", stats.Errors)
	}

	return nil
}

// processAlertByUID processes a single alert by its UID.
func (j *SignalValidationJob) processAlertByUID(ctx context.Context, alertUID string, fullLookback bool) error {
	// Get alert's PromQL
	promQL, title, err := j.alertSignalMatcher.GetAlertPromQL(ctx, alertUID)
	if err != nil {
		return fmt.Errorf("failed to get alert PromQL: %w", err)
	}

	// Determine lookback
	lookback := j.config.GetLookbackPeriod()
	if !fullLookback {
		lookback = 24 * time.Hour
	}

	// Fetch transitions
	endTime := time.Now()
	startTime := endTime.Add(-lookback)

	transitions, err := FetchStateTransitions(ctx, j.graphClient, alertUID, j.integrationName, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to fetch transitions: %w", err)
	}

	if len(transitions) == 0 {
		return nil
	}

	// Check for flapping
	if j.flappingDetector.IsFlapping(transitions) {
		j.logger.Debug("Alert %s (%s) is flapping, skipping", alertUID, title)
		return nil
	}

	// Process the alert
	stats, err := j.processAlert(ctx, alertUID, title, promQL, transitions)
	if err != nil {
		return err
	}

	j.mu.Lock()
	j.status.AlertsProcessed++
	j.status.TransitionsEvaluated += stats.transitionsEvaluated
	j.status.CorrelationsFound += stats.correlationsFound
	j.status.CorrelationsUpdated += stats.correlationsUpdated
	j.mu.Unlock()

	return nil
}

type processAlertStats struct {
	transitionsEvaluated int
	correlationsFound    int
	correlationsUpdated  int
}

// processAlert processes all transitions for a single alert.
func (j *SignalValidationJob) processAlert(
	ctx context.Context,
	alertUID string,
	alertTitle string,
	alertPromQL string,
	transitions []StateTransition,
) (processAlertStats, error) {
	stats := processAlertStats{}

	// Find matching SignalAnchors
	matches, err := j.alertSignalMatcher.FindMatchingSignals(ctx, alertUID, alertPromQL)
	if err != nil {
		return stats, fmt.Errorf("failed to find matching signals: %w", err)
	}

	if len(matches) == 0 {
		j.logger.Debug("No signal matches for alert %s (%s)", alertUID, alertTitle)
		return stats, nil
	}

	j.logger.Debug("Processing alert %s (%s) with %d matches and %d transitions",
		alertUID, alertTitle, len(matches), len(transitions))

	// Process each significant transition
	for _, transition := range transitions {
		if !j.flappingDetector.IsTransitionSignificant(transition) {
			continue
		}

		stats.transitionsEvaluated++

		// Evaluate against each matching signal
		for _, match := range matches {
			select {
			case <-ctx.Done():
				return stats, ctx.Err()
			default:
			}

			observation, err := j.processTransition(ctx, match, transition)
			if err != nil {
				j.logger.Debug("Failed to process transition for %s: %v", match.MetricName, err)
				continue
			}

			// Record the observation
			signalKey := SignalAnchorKey{
				MetricName:        match.MetricName,
				WorkloadNamespace: match.Namespace,
				WorkloadName:      match.WorkloadName,
			}

			if err := j.correlationStore.RecordObservation(
				ctx,
				signalKey,
				alertUID,
				match.WorkloadUID,
				match.WorkloadName,
				match.Namespace,
				*observation,
			); err != nil {
				j.logger.Warn("Failed to record observation: %v", err)
				continue
			}

			stats.correlationsUpdated++
			if observation.WasSignificant {
				stats.correlationsFound++
			}

			// Update aggregate score on SignalAnchor
			if err := j.correlationStore.UpdateSignalAnchorAggregateScore(ctx, signalKey); err != nil {
				j.logger.Warn("Failed to update aggregate score: %v", err)
			}
		}
	}

	return stats, nil
}

// processTransition evaluates a single alert transition against a matching signal.
func (j *SignalValidationJob) processTransition(
	ctx context.Context,
	match AlertSignalMatch,
	transition StateTransition,
) (*CorrelationObservation, error) {
	// Get metric windows around the transition
	// Default forDuration to 0 if not available
	forDuration := time.Duration(0) // TODO: Get from alert if available

	before, after, err := j.metricEvaluator.GetMetricWindows(
		ctx,
		match.MetricName,
		match.Namespace,
		transition.Timestamp,
		forDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric windows: %w", err)
	}

	// Run statistical analysis
	result := j.statisticalAnalyzer.Analyze(before, after)

	return &CorrelationObservation{
		Timestamp:      transition.Timestamp,
		WasSignificant: result.IsSignificant,
		Stats:          result.ToGraphStats(),
	}, nil
}

// setLastError updates the last error (thread-safe).
func (j *SignalValidationJob) setLastError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		j.status.LastError = err.Error()
	} else {
		j.status.LastError = ""
	}
}
