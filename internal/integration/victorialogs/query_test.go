package victorialogs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildLogsQLQuery_TimeRangeValidation(t *testing.T) {
	tests := []struct {
		name      string
		params    QueryParams
		expectEmpty bool
		description string
	}{
		{
			name: "valid range - 15 minutes",
			params: QueryParams{
				TimeRange: TimeRange{
					Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					End:   time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC),
				},
			},
			expectEmpty: false,
			description: "Should accept exactly 15-minute range",
		},
		{
			name: "valid range - 1 hour",
			params: QueryParams{
				TimeRange: TimeRange{
					Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					End:   time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
				},
			},
			expectEmpty: false,
			description: "Should accept 1-hour range",
		},
		{
			name: "invalid range - 14 minutes",
			params: QueryParams{
				TimeRange: TimeRange{
					Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					End:   time.Date(2024, 1, 1, 12, 14, 0, 0, time.UTC),
				},
			},
			expectEmpty: true,
			description: "Should reject range below 15 minutes",
		},
		{
			name: "invalid range - 1 second",
			params: QueryParams{
				TimeRange: TimeRange{
					Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					End:   time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
				},
			},
			expectEmpty: true,
			description: "Should reject very short range (1 second)",
		},
		{
			name: "zero time range - uses default",
			params: QueryParams{
				TimeRange: TimeRange{},
			},
			expectEmpty: false,
			description: "Should accept zero time range (uses default 1 hour)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := BuildLogsQLQuery(tt.params)

			if tt.expectEmpty {
				assert.Empty(t, query, tt.description)
			} else {
				assert.NotEmpty(t, query, tt.description)
				// Verify query contains time filter
				assert.Contains(t, query, "_time:", "Query should contain time filter")
			}
		})
	}
}

func TestBuildLogsQLQuery_WithFilters(t *testing.T) {
	// Test that validation doesn't break normal query construction
	// Use a time range relative to now for the test
	now := time.Now()
	params := QueryParams{
		Namespace: "prod",
		Pod:       "app-pod",
		Level:     "error",
		TimeRange: TimeRange{
			Start: now.Add(-1 * time.Hour),
			End:   now,
		},
		Limit: 100,
	}

	query := BuildLogsQLQuery(params)

	assert.NotEmpty(t, query, "Query should be constructed")
	// LogsQL uses kubernetes.* field names and field:"value" syntax (not :=)
	assert.Contains(t, query, `kubernetes.pod_namespace:"prod"`, "Query should include namespace filter with kubernetes.* prefix")
	assert.Contains(t, query, `kubernetes.pod_name:"app-pod"`, "Query should include pod filter with kubernetes.* prefix")
	assert.Contains(t, query, `level:"error"`, "Query should include level filter")
	// LogsQL uses relative time format like _time:1h or _time:60m
	assert.Contains(t, query, "_time:", "Query should include time filter")
	assert.Contains(t, query, "| limit 100", "Query should include limit")
	// Verify no AND keyword (LogsQL uses space-separated filters)
	assert.NotContains(t, query, " AND ", "Query should NOT use explicit AND keyword")
}
