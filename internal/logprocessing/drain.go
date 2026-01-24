package logprocessing

import (
	"github.com/faceair/drain"
)

// DrainConfig holds configuration for the Drain algorithm wrapper.
// These parameters control how logs are clustered into templates.
type DrainConfig struct {
	// LogClusterDepth controls the depth of the parse tree (minimum 3, recommended 4).
	// Deeper trees create more specific templates but increase memory usage.
	LogClusterDepth int

	// SimTh is the similarity threshold (0.3-0.5 for structured logs, 0.5-0.6 for unstructured).
	// Higher values merge more logs together (looser clustering).
	SimTh float64

	// MaxChildren limits branches per node to prevent explosion from variable-starting logs.
	// Recommended: 100 (prevents branch explosion while maintaining accuracy).
	MaxChildren int

	// MaxClusters limits total number of templates (0 = unlimited).
	// Set to prevent unbounded memory growth in high-volume environments.
	MaxClusters int

	// ExtraDelimiters are additional token separators beyond whitespace.
	// Common: ["_", "="] for underscore-separated and key=value patterns.
	ExtraDelimiters []string

	// ParamString is the wildcard placeholder used in templates.
	// Default: "<*>" matches Drain3 convention.
	ParamString string
}

// DefaultDrainConfig returns recommended configuration for structured Kubernetes logs.
// Research guidance: sim_th=0.4 for balanced clustering, tree depth=4 (minimum 3),
// maxChildren=100 prevents branch explosion from variable-starting logs.
func DefaultDrainConfig() DrainConfig {
	return DrainConfig{
		LogClusterDepth: 4,
		SimTh:           0.4,
		MaxChildren:     100,
		MaxClusters:     0, // Unlimited - rely on count-based pruning instead
		ExtraDelimiters: []string{"_", "="},
		ParamString:     "<*>",
	}
}

// DrainProcessor wraps the Drain algorithm with configurable parameters.
// It provides Train and Match methods for clustering logs into templates.
type DrainProcessor struct {
	drain *drain.Drain
}

// NewDrainProcessor creates a new Drain processor with the given configuration.
func NewDrainProcessor(config DrainConfig) *DrainProcessor {
	drainConfig := &drain.Config{
		LogClusterDepth: config.LogClusterDepth,
		SimTh:           config.SimTh,
		MaxChildren:     config.MaxChildren,
		MaxClusters:     config.MaxClusters,
		ExtraDelimiters: config.ExtraDelimiters,
		ParamString:     config.ParamString,
	}

	return &DrainProcessor{
		drain: drain.New(drainConfig),
	}
}

// Train processes a log message and returns the matched or newly created cluster.
// This is the primary method for ingesting logs during template extraction.
func (dp *DrainProcessor) Train(logMessage string) *drain.LogCluster {
	return dp.drain.Train(logMessage)
}

// Match finds the best matching cluster for a log message without updating the model.
// Useful for classification without affecting template training.
func (dp *DrainProcessor) Match(logMessage string) *drain.LogCluster {
	return dp.drain.Match(logMessage)
}
