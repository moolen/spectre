package victorialogs

import (
	"fmt"
	"strings"
	"time"
)

// BuildLogsQLQuery constructs a LogsQL query from structured parameters.
// Filters use exact match operator (:=) and always include a time range.
// Returns a complete LogsQL query string ready for execution.
func BuildLogsQLQuery(params QueryParams) string {
	// Validate time range meets minimum duration requirement (15 minutes per VLOG-03)
	if !params.TimeRange.IsZero() {
		if err := params.TimeRange.ValidateMinimumDuration(15 * time.Minute); err != nil {
			// Return empty query on validation failure - caller should check for empty result
			// Alternative: log warning and clamp to 15min, but explicit failure is clearer
			return ""
		}
	}

	var filters []string

	// Add K8s-focused field filters (only if non-empty)
	if params.Namespace != "" {
		filters = append(filters, fmt.Sprintf(`namespace:="%s"`, params.Namespace))
	}
	if params.Pod != "" {
		filters = append(filters, fmt.Sprintf(`pod:="%s"`, params.Pod))
	}
	if params.Container != "" {
		filters = append(filters, fmt.Sprintf(`container:="%s"`, params.Container))
	}
	if params.Level != "" {
		filters = append(filters, fmt.Sprintf(`level:="%s"`, params.Level))
	}

	// Add time range filter (always required to prevent full history scans)
	timeFilter := "_time:[1h ago, now]" // Default: last 1 hour
	if !params.TimeRange.IsZero() {
		// Use RFC3339 format (ISO 8601 compliant)
		start := params.TimeRange.Start.Format(time.RFC3339)
		end := params.TimeRange.End.Format(time.RFC3339)
		timeFilter = fmt.Sprintf("_time:[%s, %s]", start, end)
	}
	filters = append(filters, timeFilter)

	// Join filters with AND operator
	query := strings.Join(filters, " AND ")

	// Apply limit if specified
	if params.Limit > 0 {
		query = fmt.Sprintf("%s | limit %d", query, params.Limit)
	}

	return query
}

// BuildHistogramQuery constructs a LogsQL query for histogram aggregation.
// The /select/logsql/hits endpoint handles time bucketing with the 'step' parameter,
// so we only need the base query filters.
func BuildHistogramQuery(params QueryParams) string {
	return BuildLogsQLQuery(params)
}

// BuildAggregationQuery constructs a LogsQL query for aggregation by dimensions.
// Uses the 'stats' pipe to count logs grouped by specified fields.
func BuildAggregationQuery(params QueryParams, groupBy []string) string {
	baseQuery := BuildLogsQLQuery(params)

	// Build stats aggregation clause
	if len(groupBy) > 0 {
		groupByClause := strings.Join(groupBy, ", ")
		return fmt.Sprintf("%s | stats count() by %s", baseQuery, groupByClause)
	}

	// If no groupBy specified, just return count
	return fmt.Sprintf("%s | stats count()", baseQuery)
}
