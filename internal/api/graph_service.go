package api

import (
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// GraphService preserves the API package constructor surface while delegating
// graph orchestration to the app layer.
type GraphService = appgraph.Service

func NewGraphService(store analysisstore.AnalysisStore, logger *logging.Logger, tracer trace.Tracer) *GraphService {
	return appgraph.NewService(store, logger, tracer)
}

func NewGraphServiceFromGraphClient(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *GraphService {
	return appgraph.NewServiceFromGraphClient(graphClient, logger, tracer)
}
