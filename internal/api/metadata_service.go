package api

import (
	"context"
	"sort"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MetadataQueryExecutor interface for executors that support efficient metadata queries
type MetadataQueryExecutor interface {
	QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error)
}

// MetadataService contains shared business logic for metadata operations
// This service is framework-agnostic and used by REST handlers
type MetadataService struct {
	queryExecutor QueryExecutor
	metadataCache *MetadataCache
	logger        *logging.Logger
	tracer        trace.Tracer
}

// NewMetadataService creates a new metadata service
// metadataCache is optional - if nil, queries will go directly to the executor
func NewMetadataService(queryExecutor QueryExecutor, metadataCache *MetadataCache, logger *logging.Logger, tracer trace.Tracer) *MetadataService {
	return &MetadataService{
		queryExecutor: queryExecutor,
		metadataCache: metadataCache,
		logger:        logger,
		tracer:        tracer,
	}
}

// GetMetadata retrieves metadata (namespaces, kinds, time range) from cache or fresh query
func (s *MetadataService) GetMetadata(ctx context.Context, useCache bool, startTimeNs, endTimeNs int64) (*models.MetadataResponse, bool, error) {
	ctx, span := s.tracer.Start(ctx, "metadata.getMetadata")
	defer span.End()

	span.SetAttributes(
		attribute.Bool("use_cache", useCache),
		attribute.Int64("start_time_ns", startTimeNs),
		attribute.Int64("end_time_ns", endTimeNs),
	)

	// Always try to use cache first when available
	// Metadata (namespaces, kinds) changes infrequently, so returning cached data
	// provides fast responses. The cache is refreshed in the background periodically.
	// Time filtering for metadata is rarely needed since filter dropdowns need all values.
	if useCache && s.metadataCache != nil {
		s.logger.Debug("Attempting to use metadata cache")
		cachedData, err := s.metadataCache.Get()
		if err == nil {
			// Successfully got cached data - return it immediately
			span.SetAttributes(
				attribute.Bool("cache_hit", true),
				attribute.Int("namespace_count", len(cachedData.Namespaces)),
				attribute.Int("kind_count", len(cachedData.Kinds)),
			)
			s.logger.Debug("Metadata cache hit: %d namespaces, %d kinds",
				len(cachedData.Namespaces), len(cachedData.Kinds))
			return cachedData, true, nil
		}

		// Cache failed - log and fall through to direct query
		s.logger.Warn("Metadata cache unavailable, falling back to direct query: %v", err)
		span.SetAttributes(attribute.Bool("cache_hit", false))
	}

	// Try to use efficient metadata query if available
	if metadataExecutor, ok := s.queryExecutor.(MetadataQueryExecutor); ok {
		namespacesList, kindsList, minTime, maxTime, err := metadataExecutor.QueryDistinctMetadata(ctx, startTimeNs, endTimeNs)
		if err != nil {
			s.logger.Error("Failed to query metadata: %v", err)
			span.RecordError(err)
			return nil, false, err
		}

		// Convert nanoseconds to seconds for API
		if minTime < 0 {
			minTime = 0
		}
		if maxTime < 0 {
			maxTime = 0
		}

		response := &models.MetadataResponse{
			Namespaces: namespacesList,
			Kinds:      kindsList,
			TimeRange: models.TimeRangeInfo{
				Earliest: minTime / 1e9,
				Latest:   maxTime / 1e9,
			},
		}

		span.SetAttributes(
			attribute.Int("namespace_count", len(namespacesList)),
			attribute.Int("kind_count", len(kindsList)),
		)

		s.logger.Debug("Metadata query completed: %d namespaces, %d kinds",
			len(namespacesList), len(kindsList))

		return response, false, nil
	}

	// Fallback to old method (shouldn't happen with current implementations)
	s.logger.Warn("Query executor does not support QueryDistinctMetadata, using fallback")
	span.SetAttributes(attribute.Bool("fallback_query", true))

	// Use fallback via QueryDistinctMetadataFallback
	response, err := s.QueryDistinctMetadataFallback(ctx, startTimeNs/1e9, endTimeNs/1e9)
	if err != nil {
		span.RecordError(err)
		return nil, false, err
	}

	span.SetAttributes(
		attribute.Int("namespace_count", len(response.Namespaces)),
		attribute.Int("kind_count", len(response.Kinds)),
	)

	return response, false, nil
}

// QueryDistinctMetadataFallback performs a full query and extracts metadata
// This is used when the query executor doesn't support efficient metadata queries
func (s *MetadataService) QueryDistinctMetadataFallback(ctx context.Context, startTime, endTime int64) (*models.MetadataResponse, error) {
	ctx, span := s.tracer.Start(ctx, "metadata.queryDistinctMetadataFallback")
	defer span.End()

	query := &models.QueryRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := s.queryExecutor.Execute(ctx, query)
	if err != nil {
		s.logger.Error("Failed to query events in fallback: %v", err)
		span.RecordError(err)
		return nil, err
	}

	// Extract unique namespaces and kinds
	namespaces := make(map[string]bool)
	kinds := make(map[string]bool)
	minTime := int64(-1)
	maxTime := int64(-1)

	for _, event := range queryResult.Events {
		namespaces[event.Resource.Namespace] = true
		kinds[event.Resource.Kind] = true

		if minTime < 0 || event.Timestamp < minTime {
			minTime = event.Timestamp
		}
		if maxTime < 0 || event.Timestamp > maxTime {
			maxTime = event.Timestamp
		}
	}

	// Convert maps to sorted slices
	namespacesList := make([]string, 0, len(namespaces))
	for ns := range namespaces {
		namespacesList = append(namespacesList, ns)
	}
	sort.Strings(namespacesList)

	kindsList := make([]string, 0, len(kinds))
	for kind := range kinds {
		kindsList = append(kindsList, kind)
	}
	sort.Strings(kindsList)

	// Convert nanoseconds to seconds for API
	if minTime < 0 {
		minTime = 0
	}
	if maxTime < 0 {
		maxTime = 0
	}

	response := &models.MetadataResponse{
		Namespaces: namespacesList,
		Kinds:      kindsList,
		TimeRange: models.TimeRangeInfo{
			Earliest: minTime / 1e9,
			Latest:   maxTime / 1e9,
		},
	}

	s.logger.Debug("Fallback metadata extraction complete: %d namespaces, %d kinds from %d events",
		len(namespacesList), len(kindsList), len(queryResult.Events))

	return response, nil
}
