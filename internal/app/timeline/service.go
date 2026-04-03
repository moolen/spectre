package timeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/moolen/spectre/internal/api/validation"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	querySource     QuerySource
	logger          *logging.Logger
	tracer          trace.Tracer
	validator       *validation.Validator
}

func NewService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *Service {
	return &Service{
		storageExecutor: queryExecutor,
		querySource:     QuerySourceStorage,
		logger:          logger,
		validator:       validation.NewValidator(),
		tracer:          tracer,
	}
}

func NewServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource QuerySource, logger *logging.Logger, tracer trace.Tracer) *Service {
	return &Service{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		querySource:     querySource,
		logger:          logger,
		validator:       validation.NewValidator(),
		tracer:          tracer,
	}
}

func (s *Service) GetActiveExecutor() QueryExecutor {
	switch s.querySource {
	case QuerySourceGraph:
		if s.graphExecutor != nil {
			return s.graphExecutor
		}
		s.logger.Warn("Graph executor requested but not available, falling back to storage")
		return s.storageExecutor
	case QuerySourceStorage:
		return s.storageExecutor
	default:
		return s.storageExecutor
	}
}

func (s *Service) ExtractReasonFromResourceData(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", true
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(data, &resource); err != nil {
		return "", true
	}

	status, ok := resource["status"].(map[string]interface{})
	if !ok {
		return "", true
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok || len(conditions) == 0 {
		return "", true
	}

	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}
		if reason, ok := cond["reason"].(string); ok && reason != "" {
			return reason, false
		}
	}

	return "", true
}

func (s *Service) Validator() *validation.Validator {
	return s.validator
}

func (s *Service) Logger() *logging.Logger {
	return s.logger
}

func (s *Service) Tracer() trace.Tracer {
	return s.tracer
}

func (s *Service) StorageExecutor() QueryExecutor {
	return s.storageExecutor
}

func (s *Service) GraphExecutor() QueryExecutor {
	return s.graphExecutor
}

func (s *Service) QuerySource() QuerySource {
	return s.querySource
}

func (s *Service) ExecuteConcurrentQueries(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, *models.QueryResult, error) {
	ctx, span := s.tracer.Start(ctx, "timeline.executeConcurrentQueries")
	defer span.End()

	executor := s.GetActiveExecutor()
	if executor == nil {
		return nil, nil, fmt.Errorf("no query executor available")
	}

	span.SetAttributes(attribute.String("query.source", string(s.querySource)))

	var (
		resourceResult *models.QueryResult
		eventResult    *models.QueryResult
		resourceErr    error
		eventErr       error
		wg             sync.WaitGroup
	)

	eventQuery := &models.QueryRequest{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: query.Filters.GetNamespaces(),
		},
	}

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, resourceSpan := s.tracer.Start(ctx, "timeline.resourceQuery")
		defer resourceSpan.End()

		resourceResult, resourceErr = executor.Execute(ctx, query)
		if resourceErr != nil {
			resourceSpan.RecordError(resourceErr)
			resourceSpan.SetStatus(codes.Error, "Resource query failed")
		}
	}()

	go func() {
		defer wg.Done()
		_, eventSpan := s.tracer.Start(ctx, "timeline.eventQuery")
		defer eventSpan.End()

		eventResult, eventErr = executor.Execute(ctx, eventQuery)
		if eventErr != nil {
			eventSpan.RecordError(eventErr)
			eventSpan.SetStatus(codes.Error, "Event query failed")
			s.logger.Warn("Failed to fetch Kubernetes events for timeline: %v", eventErr)
		}
	}()

	wg.Wait()

	if resourceErr != nil {
		return nil, nil, resourceErr
	}

	if eventErr != nil {
		eventResult = &models.QueryResult{
			Events: []models.Event{},
		}
	}

	span.SetAttributes(
		attribute.Int("resource_count", int(resourceResult.Count)),
		attribute.Int("event_count", int(eventResult.Count)),
	)

	s.logger.Debug("Concurrent queries completed: resources=%d (%dms), events=%d (%dms)",
		resourceResult.Count, resourceResult.ExecutionTimeMs,
		eventResult.Count, eventResult.ExecutionTimeMs)

	return resourceResult, eventResult, nil
}
