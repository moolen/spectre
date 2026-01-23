package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AlertAnalysisService orchestrates historical analysis of alerts:
// - Fetches state transitions from graph
// - Computes flappiness score
// - Computes baseline and deviation
// - Categorizes alert behavior
// - Caches results with 5-minute TTL
type AlertAnalysisService struct {
	graphClient     graph.Client
	integrationName string
	cache           *expirable.LRU[string, AnalysisResult]
	logger          *logging.Logger
}

// AnalysisResult represents the complete analysis of an alert
type AnalysisResult struct {
	FlappinessScore float64           // 0.0-1.0 score from ComputeFlappinessScore
	DeviationScore  float64           // Number of standard deviations from baseline
	Baseline        StateDistribution // Historical baseline state distribution
	Categories      AlertCategories   // Multi-label categorization
	ComputedAt      time.Time         // When this analysis was performed
	DataAvailable   time.Duration     // How much history was available
}

// ErrInsufficientData indicates insufficient historical data for analysis
type ErrInsufficientData struct {
	Available time.Duration
	Required  time.Duration
}

func (e ErrInsufficientData) Error() string {
	return fmt.Sprintf("insufficient data for analysis: available %v, required %v",
		e.Available, e.Required)
}

// NewAlertAnalysisService creates a new alert analysis service
//
// Parameters:
//   - graphClient: client for querying graph database
//   - integrationName: name of Grafana integration (for scoping queries)
//   - logger: logger instance
//
// Returns:
//   - service with 1000-entry LRU cache, 5-minute TTL
func NewAlertAnalysisService(
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) *AlertAnalysisService {
	// Create cache with 1000 max entries, 5-minute TTL
	cache := expirable.NewLRU[string, AnalysisResult](1000, nil, 5*time.Minute)

	return &AlertAnalysisService{
		graphClient:     graphClient,
		integrationName: integrationName,
		cache:           cache,
		logger:          logger,
	}
}

// AnalyzeAlert performs complete historical analysis of an alert
//
// Fetches 7-day state transition history and computes:
// - Flappiness score (6-hour window)
// - Baseline comparison (7-day rolling baseline)
// - Deviation score (current vs baseline)
// - Multi-label categorization
//
// Requires at least 24 hours of history for statistically meaningful analysis.
// Results are cached for 5 minutes to handle repeated queries.
//
// Parameters:
//   - ctx: context for cancellation
//   - alertUID: unique identifier of alert
//
// Returns:
//   - AnalysisResult with all computed metrics
//   - ErrInsufficientData if < 24h history available
//   - error for graph query failures
func (s *AlertAnalysisService) AnalyzeAlert(ctx context.Context, alertUID string) (*AnalysisResult, error) {
	// Check cache first
	if cached, ok := s.cache.Get(alertUID); ok {
		s.logger.Debug("Cache hit for alert analysis %s", alertUID)
		return &cached, nil
	}

	// Fetch 7-day history
	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour)

	transitions, err := FetchStateTransitions(ctx, s.graphClient, alertUID, s.integrationName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("fetch transitions: %w", err)
	}

	// Check minimum data requirement (24h)
	if len(transitions) == 0 {
		return nil, ErrInsufficientData{
			Available: 0,
			Required:  24 * time.Hour,
		}
	}

	dataAvailable := endTime.Sub(transitions[0].Timestamp)
	if dataAvailable < 24*time.Hour {
		return nil, ErrInsufficientData{
			Available: dataAvailable,
			Required:  24 * time.Hour,
		}
	}

	// Compute flappiness (6-hour window)
	flappinessScore := ComputeFlappinessScore(transitions, 6*time.Hour, endTime)

	// Compute baseline (7-day rolling baseline)
	baseline, stdDev, err := ComputeRollingBaseline(transitions, 7, endTime)
	if err != nil {
		// Handle insufficient data error gracefully
		if _, ok := err.(*InsufficientDataError); ok {
			return nil, ErrInsufficientData{
				Available: dataAvailable,
				Required:  24 * time.Hour,
			}
		}
		return nil, fmt.Errorf("compute baseline: %w", err)
	}

	// Compute current state distribution (last 1 hour)
	recentTransitions := filterTransitions(transitions, endTime.Add(-1*time.Hour), endTime)
	currentDist := computeCurrentDistribution(recentTransitions, transitions, endTime, 1*time.Hour)

	// Compare to baseline
	deviationScore := CompareToBaseline(currentDist, baseline, stdDev)

	// Categorize alert
	categories := CategorizeAlert(transitions, endTime, flappinessScore)

	// Build result
	result := AnalysisResult{
		FlappinessScore: flappinessScore,
		DeviationScore:  deviationScore,
		Baseline:        baseline,
		Categories:      categories,
		ComputedAt:      endTime,
		DataAvailable:   dataAvailable,
	}

	// Cache result
	s.cache.Add(alertUID, result)

	s.logger.Debug("Analyzed alert %s: flappiness=%.2f, deviation=%.2f, categories=%v/%v",
		alertUID, flappinessScore, deviationScore, categories.Onset, categories.Pattern)

	return &result, nil
}

// filterTransitions filters transitions to those within a time range
func filterTransitions(transitions []StateTransition, startTime, endTime time.Time) []StateTransition {
	var filtered []StateTransition
	for _, t := range transitions {
		if !t.Timestamp.Before(startTime) && !t.Timestamp.After(endTime) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// computeCurrentDistribution computes state distribution for recent window
// using LOCF to handle gaps in data
func computeCurrentDistribution(recentTransitions []StateTransition, allTransitions []StateTransition, currentTime time.Time, windowSize time.Duration) StateDistribution {
	windowStart := currentTime.Add(-windowSize)

	// Use computeStateDurations which already implements LOCF
	durations := computeStateDurations(allTransitions, windowStart, currentTime)

	// Convert to percentages
	totalDuration := windowSize
	if totalDuration == 0 {
		return StateDistribution{PercentNormal: 1.0}
	}

	return StateDistribution{
		PercentNormal:  float64(durations["normal"]) / float64(totalDuration),
		PercentPending: float64(durations["pending"]) / float64(totalDuration),
		PercentFiring:  float64(durations["firing"]) / float64(totalDuration),
	}
}
