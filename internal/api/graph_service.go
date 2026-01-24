package api

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// GraphService provides unified access to graph analysis operations.
// It wraps existing analyzers (causal paths, anomaly detection, namespace graph)
// and provides a service layer for both REST handlers and MCP tools.
type GraphService struct {
	graphClient graph.Client
	logger      *logging.Logger
	tracer      trace.Tracer

	// Wrapped analyzers
	pathDiscoverer  *causalpaths.PathDiscoverer
	anomalyDetector *anomaly.AnomalyDetector
	namespaceAnalyzer *namespacegraph.Analyzer
}

// NewGraphService creates a new GraphService instance
func NewGraphService(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *GraphService {
	return &GraphService{
		graphClient:       graphClient,
		logger:            logger,
		tracer:            tracer,
		pathDiscoverer:    causalpaths.NewPathDiscoverer(graphClient),
		anomalyDetector:   anomaly.NewDetector(graphClient),
		namespaceAnalyzer: namespacegraph.NewAnalyzer(graphClient),
	}
}

// DiscoverCausalPaths discovers causal paths from root causes to a symptom resource
func (s *GraphService) DiscoverCausalPaths(ctx context.Context, input causalpaths.CausalPathsInput) (*causalpaths.CausalPathsResponse, error) {
	// Add tracing span
	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.discoverCausalPaths")
		defer span.End()
	}

	s.logger.Debug("GraphService: Discovering causal paths for resource %s at timestamp %d",
		input.ResourceUID, input.FailureTimestamp)

	// Delegate to the existing path discoverer
	result, err := s.pathDiscoverer.DiscoverCausalPaths(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		s.logger.Error("GraphService: Failed to discover causal paths: %v", err)
		return nil, fmt.Errorf("causal path discovery failed: %w", err)
	}

	s.logger.Debug("GraphService: Discovered %d causal paths", len(result.Paths))
	return result, nil
}

// DetectAnomalies detects anomalies in a resource's causal subgraph
func (s *GraphService) DetectAnomalies(ctx context.Context, input anomaly.DetectInput) (*anomaly.AnomalyResponse, error) {
	// Add tracing span
	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.detectAnomalies")
		defer span.End()
	}

	s.logger.Debug("GraphService: Detecting anomalies for resource %s from %d to %d",
		input.ResourceUID, input.Start, input.End)

	// Delegate to the existing anomaly detector
	result, err := s.anomalyDetector.Detect(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		s.logger.Error("GraphService: Failed to detect anomalies: %v", err)
		return nil, fmt.Errorf("anomaly detection failed: %w", err)
	}

	s.logger.Debug("GraphService: Detected %d anomalies", len(result.Anomalies))
	return result, nil
}

// AnalyzeNamespaceGraph analyzes resources and relationships in a namespace at a point in time
func (s *GraphService) AnalyzeNamespaceGraph(ctx context.Context, input namespacegraph.AnalyzeInput) (*namespacegraph.NamespaceGraphResponse, error) {
	// Add tracing span
	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.analyzeNamespaceGraph")
		defer span.End()
	}

	s.logger.Debug("GraphService: Analyzing namespace graph for %s at timestamp %d",
		input.Namespace, input.Timestamp)

	// Delegate to the existing namespace analyzer
	result, err := s.namespaceAnalyzer.Analyze(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		s.logger.Error("GraphService: Failed to analyze namespace graph: %v", err)
		return nil, fmt.Errorf("namespace graph analysis failed: %w", err)
	}

	s.logger.Debug("GraphService: Namespace graph has %d nodes and %d edges",
		result.Metadata.NodeCount, result.Metadata.EdgeCount)
	return result, nil
}
