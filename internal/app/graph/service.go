package graph

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	observatorygraph "github.com/moolen/spectre/internal/analysis/observatory_graph"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	analysisfalkor "github.com/moolen/spectre/internal/analysis/store/falkor"
	graphmodel "github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	logger *logging.Logger
	tracer trace.Tracer

	pathDiscoverer      *causalpaths.PathDiscoverer
	anomalyDetector     *anomaly.AnomalyDetector
	namespaceAnalyzer   *namespacegraph.Analyzer
	observatoryAnalyzer *observatorygraph.Analyzer
}

func NewService(store analysisstore.AnalysisStore, logger *logging.Logger, tracer trace.Tracer) *Service {
	return &Service{
		logger:            logger,
		tracer:            tracer,
		pathDiscoverer:    causalpaths.NewPathDiscoverer(store),
		anomalyDetector:   anomaly.NewDetector(store),
		namespaceAnalyzer: namespacegraph.NewAnalyzer(store),
	}
}

func NewServiceFromGraphClient(graphClient graphmodel.Client, logger *logging.Logger, tracer trace.Tracer) *Service {
	service := NewService(analysisfalkor.New(graphClient), logger, tracer)
	service.observatoryAnalyzer = observatorygraph.NewAnalyzer(graphClient)
	return service
}

func (s *Service) HasObservatoryAnalyzer() bool {
	return s != nil && s.observatoryAnalyzer != nil
}

func (s *Service) DiscoverCausalPaths(ctx context.Context, input causalpaths.CausalPathsInput) (*causalpaths.CausalPathsResponse, error) {
	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.discoverCausalPaths")
		defer span.End()
	}

	s.logger.Debug("GraphService: Discovering causal paths for resource %s at timestamp %d",
		input.ResourceUID, input.FailureTimestamp)

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

func (s *Service) DetectAnomalies(ctx context.Context, input anomaly.DetectInput) (*anomaly.AnomalyResponse, error) {
	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.detectAnomalies")
		defer span.End()
	}

	s.logger.Debug("GraphService: Detecting anomalies for resource %s from %d to %d",
		input.ResourceUID, input.Start, input.End)

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

func (s *Service) AnalyzeNamespaceGraph(ctx context.Context, input namespacegraph.AnalyzeInput) (*namespacegraph.NamespaceGraphResponse, error) {
	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.analyzeNamespaceGraph")
		defer span.End()
	}

	s.logger.Debug("GraphService: Analyzing namespace graph for %s at timestamp %d", input.Namespace, input.Timestamp)

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

func (s *Service) AnalyzeObservatoryGraph(ctx context.Context, input observatorygraph.AnalyzeInput) (*observatorygraph.ObservatoryGraphResponse, error) {
	if s == nil || s.observatoryAnalyzer == nil {
		return nil, fmt.Errorf("observatory graph analysis is not supported by the current backend")
	}

	var span trace.Span
	if s.tracer != nil {
		ctx, span = s.tracer.Start(ctx, "graph.analyzeObservatoryGraph")
		defer span.End()
	}

	s.logger.Debug("GraphService: Analyzing observatory graph for integration=%s namespace=%s",
		input.Integration, input.Namespace)

	result, err := s.observatoryAnalyzer.Analyze(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		s.logger.Error("GraphService: Failed to analyze observatory graph: %v", err)
		return nil, fmt.Errorf("observatory graph analysis failed: %w", err)
	}

	s.logger.Debug("GraphService: Observatory graph has %d nodes and %d edges",
		result.Metadata.NodeCount, result.Metadata.EdgeCount)
	return result, nil
}
