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
	params := QueryParams{
		Namespace: "prod",
		Pod:       "app-pod",
		Level:     "error",
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
		},
		Limit: 100,
	}

	query := BuildLogsQLQuery(params)

	assert.NotEmpty(t, query, "Query should be constructed")
	assert.Contains(t, query, `namespace:="prod"`, "Query should include namespace filter")
	assert.Contains(t, query, `pod:="app-pod"`, "Query should include pod filter")
	assert.Contains(t, query, `level:="error"`, "Query should include level filter")
	assert.Contains(t, query, "_time:[2024-01-01T12:00:00Z, 2024-01-01T13:00:00Z]", "Query should include time range")
	assert.Contains(t, query, "| limit 100", "Query should include limit")
}
