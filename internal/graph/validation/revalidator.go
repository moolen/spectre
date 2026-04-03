package validation

import (
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// EdgeRevalidator periodically revalidates inferred edges to ensure they remain valid
type EdgeRevalidator struct {
	client   graph.Client
	interval time.Duration
	logger   *logging.Logger

	// Configuration
	maxAge           time.Duration // Maximum age before revalidation required
	staleThreshold   time.Duration // Age after which edges are marked as stale
	decayEnabled     bool
	decayInterval6h  time.Duration
	decayInterval24h time.Duration
	decayFactor6h    float64
	decayFactor24h   float64
}

// Config holds configuration for the EdgeRevalidator
type Config struct {
	// Interval between revalidation runs
	Interval time.Duration

	// MaxAge is the maximum time since last validation before an edge needs revalidation
	MaxAge time.Duration

	// StaleThreshold is the age after which edges are marked as stale
	StaleThreshold time.Duration

	// DecayEnabled controls whether confidence decay is applied
	DecayEnabled bool

	// Decay settings
	DecayInterval6h  time.Duration
	DecayInterval24h time.Duration
	DecayFactor6h    float64
	DecayFactor24h   float64
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Interval:         5 * time.Minute,    // Run every 5 minutes
		MaxAge:           1 * time.Hour,      // Revalidate after 1 hour
		StaleThreshold:   7 * 24 * time.Hour, // Mark as stale after 7 days
		DecayEnabled:     true,
		DecayInterval6h:  6 * time.Hour,
		DecayInterval24h: 24 * time.Hour,
		DecayFactor6h:    0.9, // 10% decay
		DecayFactor24h:   0.7, // 30% decay
	}
}

// NewEdgeRevalidator creates a new edge revalidator
func NewEdgeRevalidator(client graph.Client, config Config) *EdgeRevalidator {
	return &EdgeRevalidator{
		client:           client,
		interval:         config.Interval,
		maxAge:           config.MaxAge,
		staleThreshold:   config.StaleThreshold,
		decayEnabled:     config.DecayEnabled,
		decayInterval6h:  config.DecayInterval6h,
		decayInterval24h: config.DecayInterval24h,
		decayFactor6h:    config.DecayFactor6h,
		decayFactor24h:   config.DecayFactor24h,
		logger:           logging.GetLogger("graph.validation"),
	}
}

// RevalidationStats tracks statistics for a revalidation cycle
type RevalidationStats struct {
	StartTime        time.Time
	EndTime          time.Time
	TotalEdges       int
	RevalidatedEdges int
	InvalidatedEdges int
	DecayedEdges     int
	StaleEdges       int
	UpdatedEdges     int
	ErrorCount       int
}

// GetStats returns the current revalidation statistics
func (r *EdgeRevalidator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"interval":       r.interval.String(),
		"maxAge":         r.maxAge.String(),
		"staleThreshold": r.staleThreshold.String(),
		"decayEnabled":   r.decayEnabled,
		"decayFactor6h":  r.decayFactor6h,
		"decayFactor24h": r.decayFactor24h,
	}
}
