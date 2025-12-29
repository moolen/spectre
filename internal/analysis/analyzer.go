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

// Analyze performs root cause analysis using the causality-first approach
func (a *RootCauseAnalyzer) Analyze(ctx context.Context, input AnalyzeInput) (*RootCauseAnalysisV2, error) {
	startTime := time.Now()

	// 1. Extract observed symptom (facts only, no inference)
	a.logger.Debug("Extracting observed symptom for resource %s", input.ResourceUID)
	symptom, err := a.extractObservedSymptom(ctx, input.ResourceUID, input.FailureTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to extract symptom: %w", err)
	}

	// 2. Build causal chain
	a.logger.Debug("Building causal chain from symptom")
	lookbackNs := input.LookbackNs
	if lookbackNs == 0 {
		lookbackNs = int64(600_000_000_000) // Default: 10 minutes
	}
	graph, err := a.buildCausalGraph(ctx, symptom, input.FailureTimestamp, lookbackNs)
	if err != nil {
		a.logger.Debug("Failed to build causal graph: %v, using symptom-only response", err)
		// Fallback: create minimal graph with just the symptom
		graph = CausalGraph{
			Nodes: []GraphNode{
				{
					ID: fmt.Sprintf("node-%s", symptom.Resource.UID),
					Resource: symptom.Resource,
					ChangeEvent: &ChangeEventInfo{
						EventID:       "",
						Timestamp:     symptom.ObservedAt,
						EventType:     "OBSERVED",
						ConfigChanged: false,
						StatusChanged: true,
						Description:   "Observed failure",
					},
					NodeType:   "SPINE",
					StepNumber: 1,
					Reasoning:  fmt.Sprintf("Direct observation of %s", symptom.SymptomType),
				},
			},
			Edges: []GraphEdge{},
		}
	}

	// If graph is empty, create symptom-only graph
	if len(graph.Nodes) == 0 {
		a.logger.Debug("Empty causal graph, using symptom-only response")
		graph = CausalGraph{
			Nodes: []GraphNode{
				{
					ID: fmt.Sprintf("node-%s", symptom.Resource.UID),
					Resource: symptom.Resource,
					ChangeEvent: &ChangeEventInfo{
						EventID:       "",
						Timestamp:     symptom.ObservedAt,
						EventType:     "OBSERVED",
						ConfigChanged: false,
						StatusChanged: true,
						Description:   "Observed failure",
					},
					NodeType:   "SPINE",
					StepNumber: 1,
					Reasoning:  fmt.Sprintf("Direct observation of %s", symptom.SymptomType),
				},
			},
			Edges: []GraphEdge{},
		}
	}

	// 3. Identify root cause
	a.logger.Debug("Identifying root cause from graph with %d nodes", len(graph.Nodes))
	rootCause, err := a.identifyRootCause(graph, input.FailureTimestamp)
	if err != nil {
		a.logger.Debug("Failed to identify root cause: %v, using symptom as root", err)
		// Fallback: use symptom itself as root cause
		rootCause = &RootCauseHypothesis{
			Resource: symptom.Resource,
			ChangeEvent: ChangeEventInfo{
				EventID:       "",
				Timestamp:     symptom.ObservedAt,
				EventType:     "OBSERVED",
				ConfigChanged: false,
				StatusChanged: true,
				Description:   "Observed failure",
			},
			CausationType: "DirectObservation",
			Explanation: fmt.Sprintf("%s '%s' failed with %s. No causal chain found in graph data.",
				symptom.Resource.Kind, symptom.Resource.Name, symptom.SymptomType),
			TimeLagMs: 0,
		}
	}

	// 4. Calculate confidence score
	a.logger.Debug("Calculating confidence score")
	confidence := a.calculateConfidence(symptom, graph, rootCause)

	// 5. Collect supporting evidence
	a.logger.Debug("Collecting supporting evidence")
	evidence := a.collectSupportingEvidence(graph, rootCause)

	// 6. Detect excluded alternatives
	a.logger.Debug("Detecting excluded alternatives")
	excluded := a.detectExcludedAlternatives(ctx, symptom, rootCause, input.FailureTimestamp)

	executionMs := time.Since(startTime).Milliseconds()

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
			QueryExecutionMs: executionMs,
			AlgorithmVersion: "v2.0-graph",
			ExecutedAt:       time.Now(),
		},
	}, nil
}
