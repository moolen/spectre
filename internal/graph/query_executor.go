package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// QueryExecutor executes timeline queries against the graph database.
type QueryExecutor struct {
	client Client
	logger *logging.Logger
}

// NewQueryExecutor creates a new graph-based query executor.
func NewQueryExecutor(client Client) *QueryExecutor {
	return &QueryExecutor{
		client: client,
		logger: logging.GetLogger("graph.query"),
	}
}

// Execute executes a timeline query against the graph database.
func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	result, _, err := qe.ExecutePaginated(ctx, query, query.Pagination)
	return result, err
}

// ExecutePaginated executes a paginated timeline query against the graph database.
func (qe *QueryExecutor) ExecutePaginated(
	ctx context.Context,
	query *models.QueryRequest,
	pagination *models.PaginationRequest,
) (*models.QueryResult, *models.PaginationResponse, error) {
	start := time.Now()

	if err := query.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid query: %w", err)
	}

	qe.logger.DebugWithFields("Executing graph query",
		logging.Field("start_timestamp", query.StartTimestamp),
		logging.Field("end_timestamp", query.EndTimestamp),
		logging.Field("filters", fmt.Sprintf("%v", query.Filters)))

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9
	pageSize := models.DefaultPageSize
	if pagination != nil {
		pageSize = pagination.GetPageSize()
	}

	cypherQuery := qe.buildTimelineQuery(startTimeNs, endTimeNs, query.Filters, pagination)
	result, err := qe.client.ExecuteQuery(ctx, cypherQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute graph query: %w", err)
	}

	allEvents, k8sEventsByResource := qe.parseTimelineResults(result)
	limitedEvents, nextCursor, hasMore := qe.paginateTimelineEvents(allEvents, pageSize)

	executionTime := time.Since(start)
	queryResult := &models.QueryResult{
		Events:              limitedEvents,
		Count:               int32(len(limitedEvents)),           // #nosec G115
		ExecutionTimeMs:     int32(executionTime.Milliseconds()), // #nosec G115
		QueryStartTime:      startTimeNs,
		QueryEndTime:        endTimeNs,
		K8sEventsByResource: k8sEventsByResource,
	}

	paginationResp := &models.PaginationResponse{
		NextCursor: nextCursor,
		HasMore:    hasMore,
		PageSize:   pageSize,
	}

	return queryResult, paginationResp, nil
}

// SetSharedCache is a no-op for graph executor (storage-only feature).
func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
}
