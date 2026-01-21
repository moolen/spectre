package api

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SearchService contains shared business logic for search operations
// This service is framework-agnostic and used by both REST handlers and MCP tools
type SearchService struct {
	queryExecutor QueryExecutor
	logger        *logging.Logger
	tracer        trace.Tracer
	validator     *Validator
}

// NewSearchService creates a new search service
func NewSearchService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *SearchService {
	return &SearchService{
		queryExecutor: queryExecutor,
		logger:        logger,
		validator:     NewValidator(),
		tracer:        tracer,
	}
}

// ParseSearchQuery parses and validates query parameters into a QueryRequest
func (s *SearchService) ParseSearchQuery(q string, startStr, endStr string, filters map[string]string) (*models.QueryRequest, error) {
	// Validate query string is not empty
	if q == "" {
		return nil, NewValidationError("query parameter 'q' is required")
	}

	// Parse timestamps
	start, err := ParseTimestamp(startStr, "start")
	if err != nil {
		return nil, err
	}

	end, err := ParseTimestamp(endStr, "end")
	if err != nil {
		return nil, err
	}

	// Validate timestamp range
	if start < 0 || end < 0 {
		return nil, NewValidationError("timestamps must be non-negative")
	}
	if start > end {
		return nil, NewValidationError("start timestamp must be less than or equal to end timestamp")
	}

	// Build filters from query parameters
	queryFilters := models.QueryFilters{
		Group:     filters["group"],
		Version:   filters["version"],
		Kind:      filters["kind"],
		Namespace: filters["namespace"],
	}

	// Validate filters
	if err := s.validator.ValidateFilters(queryFilters); err != nil {
		return nil, err
	}

	// Build query request
	queryRequest := &models.QueryRequest{
		StartTimestamp: start,
		EndTimestamp:   end,
		Filters:        queryFilters,
	}

	// Validate complete query
	if err := queryRequest.Validate(); err != nil {
		return nil, err
	}

	return queryRequest, nil
}

// ExecuteSearch executes a search query and returns the results
func (s *SearchService) ExecuteSearch(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	// Create tracing span
	ctx, span := s.tracer.Start(ctx, "search.execute")
	defer span.End()

	// Log query execution
	s.logger.Debug("Executing search query: start=%d, end=%d, filters=%s",
		query.StartTimestamp, query.EndTimestamp, query.Filters.String())

	// Add span attributes
	span.SetAttributes(
		attribute.Int64("query.start", query.StartTimestamp),
		attribute.Int64("query.end", query.EndTimestamp),
		attribute.String("query.filters", query.Filters.String()),
	)

	// Execute query
	result, err := s.queryExecutor.Execute(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Query execution failed")
		s.logger.Error("Search query execution failed: %v", err)
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	// Add result attributes to span
	span.SetAttributes(
		attribute.Int("result.event_count", len(result.Events)),
	)

	s.logger.Debug("Search query completed: events=%d, executionTime=%dms",
		len(result.Events), result.ExecutionTimeMs)

	return result, nil
}

// BuildSearchResponse transforms QueryResult into SearchResponse
// Groups events by resource UID and extracts resource information
// TODO: Reimplement ResourceBuilder functionality for graph-based queries
func (s *SearchService) BuildSearchResponse(queryResult *models.QueryResult) (*models.SearchResponse, error) {
	// Build resources directly from events (simplified version)
	resourceMap := make(map[string]*models.Resource)
	for _, event := range queryResult.Events {
		resourceID := fmt.Sprintf("%s/%s/%s/%s", event.Resource.Group, event.Resource.Version, event.Resource.Kind, event.Resource.UID)
		if _, exists := resourceMap[resourceID]; !exists {
			resourceMap[resourceID] = &models.Resource{
				ID:        resourceID,
				Group:     event.Resource.Group,
				Version:   event.Resource.Version,
				Kind:      event.Resource.Kind,
				Namespace: event.Resource.Namespace,
				Name:      event.Resource.Name,
			}
		}
	}

	// Convert map to slice
	resources := make([]models.Resource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		resources = append(resources, *resource)
	}

	return &models.SearchResponse{
		Resources:       resources,
		Count:           len(resources),
		ExecutionTimeMs: int64(queryResult.ExecutionTimeMs),
	}, nil
}
