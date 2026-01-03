package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// RootCauseAnalyzer performs graph-based root cause analysis
type RootCauseAnalyzer struct {
	graphClient graph.Client
	logger      *logging.Logger
}

// NewRootCauseAnalyzer creates a new analyzer instance
func NewRootCauseAnalyzer(graphClient graph.Client) *RootCauseAnalyzer {
	return &RootCauseAnalyzer{
		graphClient: graphClient,
		logger:      logging.GetLogger("analysis.root_cause"),
	}
}

// AnalyzeInput defines input parameters for root cause analysis
type AnalyzeInput struct {
	ResourceUID      string
	FailureTimestamp int64 // Unix nanoseconds
	LookbackNs       int64 // Lookback window in nanoseconds (default: 10 minutes)
	MaxDepth         int
	MinConfidence    float64
}

// Analyze performs root cause analysis using the causality-first approach.
//
// This function implements a fault-tolerant analysis pipeline that degrades gracefully:
// - If the causal graph cannot be built, returns symptom-only result
// - If root cause cannot be identified, uses symptom as root cause
// - Tracks all degradation reasons and warnings in ResultQuality
//
// The ResultQuality field in QueryMetadata should be checked to determine if the
// result is degraded or contains partial data.
func (a *RootCauseAnalyzer) Analyze(ctx context.Context, input AnalyzeInput) (*RootCauseAnalysisV2, error) {
	startTime := time.Now()

	// Initialize result quality tracking
	quality := ResultQuality{
		IsDegraded:         false,
		DegradationReasons: []string{},
		IsSymptomOnly:      false,
		HasPartialData:     false,
		Warnings:           []string{},
	}

	// Initialize performance metrics
	perfMetrics := &PerformanceMetrics{
		SlowOperations: []SlowOperation{},
	}

	// 1. Extract observed symptom (facts only, no inference)
	a.logger.Debug("Extracting observed symptom for resource %s", input.ResourceUID)
	symptomStart := time.Now()
	symptom, err := a.extractObservedSymptom(ctx, input.ResourceUID, input.FailureTimestamp)
	if err != nil {
		a.logger.Error("Failed to extract symptom for resource %s: %v", input.ResourceUID, err)
		return nil, fmt.Errorf("failed to extract symptom: %w", err)
	}
	a.logger.Debug("Symptom extraction completed in %v", time.Since(symptomStart))

	// 2. Build causal chain
	a.logger.Debug("Building causal chain from symptom")
	lookbackNs := input.LookbackNs
	if lookbackNs == 0 {
		lookbackNs = DefaultLookbackNs
	}
	a.logger.Debug("Using lookback window: %v (%d ns)", time.Duration(lookbackNs), lookbackNs)

	graphStart := time.Now()
	graph, graphErr := a.buildCausalGraph(ctx, symptom, input.FailureTimestamp, lookbackNs)
	graphDuration := time.Since(graphStart)
	perfMetrics.GraphBuildDurationMs = graphDuration.Milliseconds()

	if graphErr != nil {
		// Graph building failed - this is a degraded result
		a.logger.Warn("Failed to build causal graph: %v - using symptom-only response", graphErr)
		quality.IsDegraded = true
		quality.IsSymptomOnly = true
		quality.DegradationReasons = append(quality.DegradationReasons, "graph_build_failed")
		quality.Warnings = append(quality.Warnings, fmt.Sprintf("Could not build causal graph: %v", graphErr))
		// Fallback: create minimal graph with just the symptom
		graph = createSymptomOnlyGraph(symptom)
	} else {
		a.logger.Debug("Causal graph built successfully in %v with %d nodes and %d edges",
			graphDuration, len(graph.Nodes), len(graph.Edges))

		// Check for slow graph building
		if graphDuration.Milliseconds() > SlowGraphBuildThresholdMs {
			a.logger.Warn("Slow graph building: %v (threshold: %dms)", graphDuration, SlowGraphBuildThresholdMs)
			perfMetrics.SlowOperations = append(perfMetrics.SlowOperations, SlowOperation{
				Operation:   "graph_build",
				DurationMs:  graphDuration.Milliseconds(),
				ThresholdMs: SlowGraphBuildThresholdMs,
			})
		}

		// Check if graph is empty
		if isGraphEmpty(graph) {
			a.logger.Warn("Empty causal graph returned - using symptom-only response")
			quality.IsDegraded = true
			quality.IsSymptomOnly = true
			quality.DegradationReasons = append(quality.DegradationReasons, "empty_graph")
			quality.Warnings = append(quality.Warnings, "No causal chain found for symptom")
			graph = createSymptomOnlyGraph(symptom)
		}
	}

	// 3. Identify root cause
	a.logger.Debug("Identifying root cause from graph with %d nodes", len(graph.Nodes))
	rootCauseStart := time.Now()
	rootCause, rootErr := a.identifyRootCause(graph, input.FailureTimestamp)
	rootCauseDuration := time.Since(rootCauseStart)

	if rootErr != nil {
		// Root cause identification failed - use symptom as fallback
		a.logger.Warn("Failed to identify root cause: %v - using symptom as root cause", rootErr)
		quality.IsDegraded = true
		quality.DegradationReasons = append(quality.DegradationReasons, "root_cause_identification_failed")
		quality.Warnings = append(quality.Warnings, fmt.Sprintf("Could not identify root cause: %v", rootErr))
		// Fallback: use symptom itself as root cause
		rootCause = createSymptomOnlyRootCause(symptom)
	} else {
		a.logger.Debug("Root cause identified in %v: %s/%s", rootCauseDuration,
			rootCause.Resource.Kind, rootCause.Resource.Name)
	}

	// 4. Calculate confidence score
	a.logger.Debug("Calculating confidence score")
	confidence := a.calculateConfidence(symptom, graph, rootCause)
	a.logger.Debug("Confidence score: %.2f", confidence.Score)

	// 5. Collect supporting evidence
	a.logger.Debug("Collecting supporting evidence")
	evidence := a.collectSupportingEvidence(graph, rootCause)
	a.logger.Debug("Collected %d evidence items", len(evidence))

	// 6. Detect excluded alternatives
	a.logger.Debug("Detecting excluded alternatives")
	excluded := a.detectExcludedAlternatives(ctx, symptom, rootCause, input.FailureTimestamp)
	a.logger.Debug("Found %d excluded alternatives", len(excluded))

	totalDuration := time.Since(startTime)
	perfMetrics.TotalDurationMs = totalDuration.Milliseconds()

	// Check for slow overall analysis
	if totalDuration.Milliseconds() > SlowAnalysisThresholdMs {
		a.logger.Warn("Slow analysis: %v (threshold: %dms)", totalDuration, SlowAnalysisThresholdMs)
		perfMetrics.SlowOperations = append(perfMetrics.SlowOperations, SlowOperation{
			Operation:   "full_analysis",
			DurationMs:  totalDuration.Milliseconds(),
			ThresholdMs: SlowAnalysisThresholdMs,
		})
	}

	a.logger.Info("Analysis completed in %v - degraded=%v, symptom_only=%v, confidence=%.2f",
		totalDuration, quality.IsDegraded, quality.IsSymptomOnly, confidence.Score)

	return &RootCauseAnalysisV2{
		Incident: IncidentAnalysis{
			ObservedSymptom: *symptom,
			Graph:           graph,
			RootCause:       *rootCause,
			Confidence:      confidence,
		},
		SupportingEvidence:   evidence,
		ExcludedAlternatives: excluded,
		QueryMetadata: QueryMetadata{
			QueryExecutionMs:   totalDuration.Milliseconds(),
			AlgorithmVersion:   "v2.0-graph",
			ExecutedAt:         time.Now(),
			ResultQuality:      quality,
			PerformanceMetrics: perfMetrics,
		},
	}, nil
}
