package timeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/moolen/spectre/internal/api/parsing"
	"github.com/moolen/spectre/internal/api/validation"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
)

func (s *Service) ParseQueryParameters(ctx context.Context, startStr, endStr string, filterParams map[string][]string) (*models.QueryRequest, error) {
	ctx, span := s.tracer.Start(ctx, "timeline.parseQueryParameters")
	defer span.End()

	start, err := parsing.ParseTimestamp(startStr, "start")
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	end, err := parsing.ParseTimestamp(endStr, "end")
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	if start < 0 || end < 0 {
		err := validation.NewValidationError("timestamps must be non-negative")
		span.RecordError(err)
		return nil, err
	}
	if start > end {
		err := validation.NewValidationError("start timestamp must be less than or equal to end timestamp")
		span.RecordError(err)
		return nil, err
	}

	kinds := parseMultiValueParam(filterParams, "kind", "kinds")
	namespaces := parseMultiValueParam(filterParams, "namespace", "namespaces")

	filters := models.QueryFilters{
		Group:      getSingleParam(filterParams, "group"),
		Version:    getSingleParam(filterParams, "version"),
		Kinds:      kinds,
		Namespaces: namespaces,
	}

	if err := s.validator.ValidateFilters(filters); err != nil {
		span.RecordError(err)
		return nil, err
	}

	queryRequest := &models.QueryRequest{
		StartTimestamp: start,
		EndTimestamp:   end,
		Filters:        filters,
	}

	if err := queryRequest.Validate(); err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(
		attribute.Int64("query.start", start),
		attribute.Int64("query.end", end),
		attribute.StringSlice("query.kinds", kinds),
		attribute.StringSlice("query.namespaces", namespaces),
	)

	s.logger.Debug("Parsed query parameters: start=%d, end=%d, kinds=%v, namespaces=%v",
		start, end, kinds, namespaces)

	return queryRequest, nil
}

func (s *Service) ParsePagination(pageSizeParam, cursor string, maxPageSize int) *models.PaginationRequest {
	pageSize := parseIntOrDefault(pageSizeParam, models.DefaultPageSize)
	if maxPageSize > 0 && pageSize > maxPageSize {
		s.logger.Debug("Requested page size %d exceeds maximum %d, capping to maximum", pageSize, maxPageSize)
		pageSize = maxPageSize
	}

	return &models.PaginationRequest{
		PageSize: pageSize,
		Cursor:   cursor,
	}
}

func parseMultiValueParam(params map[string][]string, singularName, pluralName string) []string {
	values := params[singularName]
	if len(values) > 0 {
		return values
	}

	if pluralCSV, ok := params[pluralName]; ok && len(pluralCSV) > 0 && pluralCSV[0] != "" {
		return strings.Split(pluralCSV[0], ",")
	}

	return nil
}

func getSingleParam(params map[string][]string, name string) string {
	if values, ok := params[name]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

func parseIntOrDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var val int
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return defaultVal
	}
	return val
}
