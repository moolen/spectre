package victorialogs

import (
	"fmt"
	"strings"
	"time"
)

// BuildLogsQLQuery constructs a LogsQL query from structured parameters.
// Filters use exact match operator (field:"value") and always include a time range.
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

	// Add K8s-focused field filters using kubernetes.* field names from Vector/Fluent Bit
	// LogsQL uses field:"value" for exact match (not := operator)
	if params.Namespace != "" {
		filters = append(filters, fmt.Sprintf(`kubernetes.pod_namespace:"%s"`, params.Namespace))
	}
	if params.Pod != "" {
		filters = append(filters, fmt.Sprintf(`kubernetes.pod_name:"%s"`, params.Pod))
	}
	if params.Container != "" {
		filters = append(filters, fmt.Sprintf(`kubernetes.container_name:"%s"`, params.Container))
	}
	if params.Level != "" {
		filters = append(filters, fmt.Sprintf(`level:"%s"`, params.Level))
	}
	// RegexMatch takes precedence - uses _msg field with regex operator
	// This is used for complex severity classification patterns
	if params.RegexMatch != "" {
		filters = append(filters, fmt.Sprintf(`_msg:~"%s"`, params.RegexMatch))
	} else if params.TextMatch != "" {
		// TextMatch performs case-insensitive word search in the log message
		// This is useful when logs don't have structured level fields
		filters = append(filters, params.TextMatch)
	}

	// Add time range filter (always required to prevent full history scans)
	// LogsQL uses _time:duration for relative time (e.g., _time:1h) or
	// _time:[start, end] with Unix timestamps or RFC3339 without spaces
	timeFilter := "_time:1h" // Default: last 1 hour
	if !params.TimeRange.IsZero() {
		// Calculate duration for relative time filter (more reliable than absolute timestamps)
		duration := time.Since(params.TimeRange.Start)
		// Round up to nearest minute for cleaner queries
		durationMins := int(duration.Minutes()) + 1
		if durationMins < 60 {
			timeFilter = fmt.Sprintf("_time:%dm", durationMins)
		} else {
			durationHours := (durationMins / 60) + 1
			timeFilter = fmt.Sprintf("_time:%dh", durationHours)
		}
	}
	filters = append(filters, timeFilter)

	// Join filters with space (LogsQL uses space for AND, not explicit "AND" keyword)
	query := strings.Join(filters, " ")

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
// LogsQL stats syntax: stats by (field1, field2) count() result_name
func BuildAggregationQuery(params QueryParams, groupBy []string) string {
	baseQuery := BuildLogsQLQuery(params)

	// Build stats aggregation clause
	// LogsQL syntax: stats by (field1, field2) count() logs
	if len(groupBy) > 0 {
		// Map simple field names to kubernetes.* field names
		mappedFields := make([]string, len(groupBy))
		for i, field := range groupBy {
			mappedFields[i] = mapFieldName(field)
		}
		groupByClause := strings.Join(mappedFields, ", ")
		return fmt.Sprintf("%s | stats by (%s) count() logs", baseQuery, groupByClause)
	}

	// If no groupBy specified, just return count
	return fmt.Sprintf("%s | stats count() logs", baseQuery)
}

// mapFieldName maps simple field names to their kubernetes.* equivalents
func mapFieldName(field string) string {
	switch field {
	case "namespace":
		return "kubernetes.pod_namespace"
	case "pod":
		return "kubernetes.pod_name"
	case "container":
		return "kubernetes.container_name"
	default:
		return field
	}
}
