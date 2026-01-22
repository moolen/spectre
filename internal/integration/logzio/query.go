package logzio

import (
	"fmt"
	"strings"
	"time"
)

// BuildLogsQuery constructs an Elasticsearch DSL query from structured parameters.
// Returns a map that can be marshaled to JSON for the Logz.io /v1/search endpoint.
func BuildLogsQuery(params QueryParams) map[string]interface{} {
	// Use default time range if not specified
	timeRange := params.TimeRange
	if timeRange.IsZero() {
		now := time.Now()
		timeRange = TimeRange{
			Start: now.Add(-1 * time.Hour),
			End:   now,
		}
	}

	// Build bool query with must clauses
	mustClauses := []map[string]interface{}{}

	// Time range filter on @timestamp field
	mustClauses = append(mustClauses, map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"gte": timeRange.Start.Format(time.RFC3339),
				"lte": timeRange.End.Format(time.RFC3339),
			},
		},
	})

	// Namespace filter (exact match with .keyword suffix)
	if params.Namespace != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"kubernetes.namespace.keyword": params.Namespace,
			},
		})
	}

	// Pod filter (exact match with .keyword suffix)
	if params.Pod != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"kubernetes.pod_name.keyword": params.Pod,
			},
		})
	}

	// Container filter (exact match with .keyword suffix)
	if params.Container != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"kubernetes.container_name.keyword": params.Container,
			},
		})
	}

	// Level filter (exact match with .keyword suffix)
	if params.Level != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"level.keyword": params.Level,
			},
		})
	}

	// RegexMatch filter on message field
	if params.RegexMatch != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"regexp": map[string]interface{}{
				"message": map[string]interface{}{
					"value":            params.RegexMatch,
					"flags":            "ALL",
					"case_insensitive": true,
				},
			},
		})
	}

	// Set default limit if not specified
	limit := params.Limit
	if limit == 0 {
		limit = 100 // Default limit
	}

	// Construct full query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": mustClauses,
			},
		},
		"size": limit,
		"sort": []map[string]interface{}{
			{
				"@timestamp": map[string]interface{}{
					"order": "desc",
				},
			},
		},
	}

	return query
}

// BuildAggregationQuery constructs an Elasticsearch DSL aggregation query.
// Returns a map that can be marshaled to JSON for the Logz.io /v1/search endpoint.
func BuildAggregationQuery(params QueryParams, groupByFields []string) map[string]interface{} {
	// Use default time range if not specified
	timeRange := params.TimeRange
	if timeRange.IsZero() {
		now := time.Now()
		timeRange = TimeRange{
			Start: now.Add(-1 * time.Hour),
			End:   now,
		}
	}

	// Build bool query with must clauses (same as BuildLogsQuery)
	mustClauses := []map[string]interface{}{}

	// Time range filter on @timestamp field
	mustClauses = append(mustClauses, map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"gte": timeRange.Start.Format(time.RFC3339),
				"lte": timeRange.End.Format(time.RFC3339),
			},
		},
	})

	// Namespace filter (exact match with .keyword suffix)
	if params.Namespace != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"kubernetes.namespace.keyword": params.Namespace,
			},
		})
	}

	// Pod filter (exact match with .keyword suffix)
	if params.Pod != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"kubernetes.pod_name.keyword": params.Pod,
			},
		})
	}

	// Container filter (exact match with .keyword suffix)
	if params.Container != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"kubernetes.container_name.keyword": params.Container,
			},
		})
	}

	// Level filter (exact match with .keyword suffix)
	if params.Level != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"level.keyword": params.Level,
			},
		})
	}

	// RegexMatch filter on message field
	if params.RegexMatch != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"regexp": map[string]interface{}{
				"message": map[string]interface{}{
					"value":            params.RegexMatch,
					"flags":            "ALL",
					"case_insensitive": true,
				},
			},
		})
	}

	// Build aggregations
	aggs := map[string]interface{}{}
	if len(groupByFields) > 0 {
		// Use first field for aggregation (typically namespace or level)
		field := groupByFields[0]

		// Append .keyword suffix for exact aggregation
		fieldWithSuffix := field
		if !strings.HasSuffix(field, ".keyword") {
			fieldWithSuffix = field + ".keyword"
		}

		aggs[field] = map[string]interface{}{
			"terms": map[string]interface{}{
				"field": fieldWithSuffix,
				"size":  1000, // Logz.io max for aggregations
				"order": map[string]interface{}{
					"_count": "desc",
				},
			},
		}
	}

	// Construct full query with size: 0 (no hits, only aggregations)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": mustClauses,
			},
		},
		"size": 0, // No hits, only aggregations
		"aggs": aggs,
	}

	return query
}

// ValidateQueryParams validates query parameters for common issues.
// Validates internal regex patterns used by overview tool for severity detection.
func ValidateQueryParams(params QueryParams) error {
	// Check for leading wildcard in RegexMatch (performance issue for Elasticsearch)
	if params.RegexMatch != "" {
		if strings.HasPrefix(params.RegexMatch, "*") || strings.HasPrefix(params.RegexMatch, "?") {
			return fmt.Errorf("leading wildcard queries are not supported by Logz.io - try suffix wildcards or remove wildcard")
		}
	}

	// Enforce max limit
	if params.Limit > 500 {
		return fmt.Errorf("limit cannot exceed 500 (requested: %d)", params.Limit)
	}

	return nil
}
