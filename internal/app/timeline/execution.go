package timeline

import (
	"context"
	"fmt"
	"sort"

	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// PaginatedExecutor extends a timeline query executor with native pagination.
type PaginatedExecutor interface {
	ExecutePaginated(context.Context, *models.QueryRequest, *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error)
}

// ExecutionResult contains the stream-ready result of a timeline query.
type ExecutionResult struct {
	ResourceResult *models.QueryResult
	EventResult    *models.QueryResult
	Index          *TimelineIndex
	Entries        []*TimelineResourceEntry
	Pagination     *models.PaginationResponse
}

// ExecuteTimeline executes a timeline query and returns a canonical application-level result.
func (s *Service) ExecuteTimeline(ctx context.Context, query *models.QueryRequest, pagination *models.PaginationRequest) (*ExecutionResult, error) {
	ctx, span := s.tracer.Start(ctx, "timeline.executeTimeline")
	defer span.End()

	executor := s.GetActiveExecutor()
	if executor == nil {
		err := fmt.Errorf("no query executor available")
		span.RecordError(err)
		span.SetStatus(codes.Error, "No executor available")
		return nil, err
	}

	span.SetAttributes(attribute.String("query.source", string(s.querySource)))

	var (
		resourceResult *models.QueryResult
		eventResult    *models.QueryResult
		paginationResp *models.PaginationResponse
		err            error
	)

	if paginatedExec, ok := executor.(PaginatedExecutor); ok && pagination != nil {
		resourceResult, paginationResp, err = paginatedExec.ExecutePaginated(ctx, query, pagination)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Paginated resource query failed")
			return nil, err
		}

		eventResult, err = s.executeEventQuery(ctx, executor, query)
		if err != nil {
			s.logger.Warn("Failed to fetch Kubernetes events for timeline: %v", err)
			eventResult = &models.QueryResult{Events: []models.Event{}}
		}
	} else {
		resourceResult, eventResult, err = s.ExecuteConcurrentQueries(ctx, query)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Timeline execution failed")
			return nil, err
		}
	}

	index := s.BuildTimelineIndex(resourceResult, eventResult)
	entries := index.Entries()

	if pagination != nil && (paginationResp == nil || paginationResp.NextCursor == "") {
		entries, paginationResp, err = s.paginateEntries(entries, pagination)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Timeline pagination failed")
			return nil, err
		}
	}

	return &ExecutionResult{
		ResourceResult: resourceResult,
		EventResult:    eventResult,
		Index:          index,
		Entries:        entries,
		Pagination:     paginationResp,
	}, nil
}

func (s *Service) executeEventQuery(ctx context.Context, executor QueryExecutor, query *models.QueryRequest) (*models.QueryResult, error) {
	eventQuery := &models.QueryRequest{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: query.Filters.GetNamespaces(),
		},
	}

	return executor.Execute(ctx, eventQuery)
}

func (s *Service) paginateEntries(entries []*TimelineResourceEntry, pagination *models.PaginationRequest) ([]*TimelineResourceEntry, *models.PaginationResponse, error) {
	pageSize := pagination.GetPageSize()

	cursor, err := models.DecodeCursor(pagination.Cursor)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor: %w", err)
	}

	sortedEntries := append([]*TimelineResourceEntry(nil), entries...)
	sortTimelineEntries(sortedEntries)

	startIdx := 0
	if cursor != nil {
		for i, entry := range sortedEntries {
			if resourceIdentityLess(entry.Kind, entry.Namespace, entry.Name, cursor.Kind, cursor.Namespace, cursor.Name) {
				continue
			}
			if entry.Kind == cursor.Kind && entry.Namespace == cursor.Namespace && entry.Name == cursor.Name {
				continue
			}
			startIdx = i
			break
		}
		if len(sortedEntries) > 0 {
			lastEntry := sortedEntries[len(sortedEntries)-1]
			if !resourceIdentityLess(cursor.Kind, cursor.Namespace, cursor.Name, lastEntry.Kind, lastEntry.Namespace, lastEntry.Name) {
				startIdx = len(sortedEntries)
			}
		}
	}

	endIdx := startIdx + pageSize
	hasMore := endIdx < len(sortedEntries)
	if endIdx > len(sortedEntries) {
		endIdx = len(sortedEntries)
	}

	pageEntries := sortedEntries[startIdx:endIdx]

	var nextCursor string
	if hasMore && len(pageEntries) > 0 {
		lastEntry := pageEntries[len(pageEntries)-1]
		nextCursor = models.NewResourceCursor(lastEntry.Kind, lastEntry.Namespace, lastEntry.Name).Encode()
	}

	return pageEntries, &models.PaginationResponse{
		HasMore:    hasMore,
		NextCursor: nextCursor,
		PageSize:   pageSize,
	}, nil
}

func resourceIdentityLess(kindA, namespaceA, nameA, kindB, namespaceB, nameB string) bool {
	if kindA != kindB {
		return kindA < kindB
	}
	if namespaceA != namespaceB {
		return namespaceA < namespaceB
	}
	return nameA < nameB
}

func sortTimelineEntries(entries []*TimelineResourceEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return resourceIdentityLess(
			entries[i].Kind, entries[i].Namespace, entries[i].Name,
			entries[j].Kind, entries[j].Namespace, entries[j].Name,
		)
	})
}
