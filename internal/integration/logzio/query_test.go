package logzio

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildLogsQuery(t *testing.T) {
	// Test basic query with time range
	params := QueryParams{
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		Limit: 50,
	}

	query := BuildLogsQuery(params)

	// Verify query structure
	if query["size"] != 50 {
		t.Errorf("Expected size 50, got %v", query["size"])
	}

	// Verify bool query exists
	queryObj, ok := query["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected query object")
	}

	boolObj, ok := queryObj["bool"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected bool object")
	}

	mustClauses, ok := boolObj["must"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected must clauses array")
	}

	// Should have time range clause
	if len(mustClauses) < 1 {
		t.Fatal("Expected at least time range clause")
	}

	// Verify time range clause
	rangeClause := mustClauses[0]
	if _, ok := rangeClause["range"]; !ok {
		t.Errorf("Expected range clause, got %+v", rangeClause)
	}

	// Verify sort by @timestamp desc
	sortArr, ok := query["sort"].([]map[string]interface{})
	if !ok || len(sortArr) == 0 {
		t.Fatal("Expected sort array")
	}

	if _, ok := sortArr[0]["@timestamp"]; !ok {
		t.Errorf("Expected sort by @timestamp, got %+v", sortArr[0])
	}
}

func TestBuildLogsQueryWithFilters(t *testing.T) {
	// Test query with all filters
	params := QueryParams{
		Namespace: "prod",
		Pod:       "api-server-123",
		Container: "api",
		Level:     "error",
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		Limit: 100,
	}

	query := BuildLogsQuery(params)

	// Marshal to JSON for inspection
	queryJSON, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	queryStr := string(queryJSON)

	// Verify .keyword suffix is present for exact-match fields
	expectedKeywords := []string{
		"kubernetes.namespace.keyword",
		"kubernetes.pod_name.keyword",
		"kubernetes.container_name.keyword",
		"level.keyword",
	}

	for _, keyword := range expectedKeywords {
		if !contains(queryStr, keyword) {
			t.Errorf("Expected query to contain %q, got:\n%s", keyword, queryStr)
		}
	}

	// Verify filter values are present
	expectedValues := []string{
		"prod",
		"api-server-123",
		"api",
		"error",
	}

	for _, value := range expectedValues {
		if !contains(queryStr, value) {
			t.Errorf("Expected query to contain value %q, got:\n%s", value, queryStr)
		}
	}
}

func TestBuildLogsQueryTimeRange(t *testing.T) {
	// Test time range formatting
	params := QueryParams{
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			End:   time.Date(2024, 1, 15, 11, 30, 45, 0, time.UTC),
		},
	}

	query := BuildLogsQuery(params)

	// Marshal to JSON
	queryJSON, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	queryStr := string(queryJSON)

	// Verify RFC3339 time format
	expectedStart := "2024-01-15T10:30:45Z"
	expectedEnd := "2024-01-15T11:30:45Z"

	if !contains(queryStr, expectedStart) {
		t.Errorf("Expected query to contain start time %q, got:\n%s", expectedStart, queryStr)
	}

	if !contains(queryStr, expectedEnd) {
		t.Errorf("Expected query to contain end time %q, got:\n%s", expectedEnd, queryStr)
	}
}

func TestBuildLogsQueryRegexMatch(t *testing.T) {
	// Test regex match clause
	params := QueryParams{
		RegexMatch: "(?i)(ERROR|Exception)",
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		},
	}

	query := BuildLogsQuery(params)

	// Marshal to JSON
	queryJSON, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	queryStr := string(queryJSON)

	// Verify regexp clause structure
	if !contains(queryStr, "regexp") {
		t.Errorf("Expected query to contain 'regexp', got:\n%s", queryStr)
	}

	if !contains(queryStr, "message") {
		t.Errorf("Expected query to contain 'message' field, got:\n%s", queryStr)
	}

	if !contains(queryStr, "(?i)(ERROR|Exception)") {
		t.Errorf("Expected query to contain regex pattern, got:\n%s", queryStr)
	}

	if !contains(queryStr, "case_insensitive") {
		t.Errorf("Expected query to contain 'case_insensitive', got:\n%s", queryStr)
	}
}

func TestBuildLogsQueryDefaultLimit(t *testing.T) {
	// Test default limit when not specified
	params := QueryParams{
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		},
		// Limit not specified
	}

	query := BuildLogsQuery(params)

	// Should default to 100
	if query["size"] != 100 {
		t.Errorf("Expected default size 100, got %v", query["size"])
	}
}

func TestBuildAggregationQuery(t *testing.T) {
	// Test aggregation query with groupBy
	params := QueryParams{
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		},
	}

	groupByFields := []string{"kubernetes.namespace"}

	query := BuildAggregationQuery(params, groupByFields)

	// Verify size is 0 (no hits, only aggregations)
	if query["size"] != 0 {
		t.Errorf("Expected size 0 for aggregation query, got %v", query["size"])
	}

	// Verify aggregations exist
	aggs, ok := query["aggs"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected aggs object")
	}

	// Verify aggregation on namespace field
	namespaceAgg, ok := aggs["kubernetes.namespace"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected kubernetes.namespace aggregation")
	}

	terms, ok := namespaceAgg["terms"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected terms aggregation")
	}

	// Verify .keyword suffix is added
	field, ok := terms["field"].(string)
	if !ok || field != "kubernetes.namespace.keyword" {
		t.Errorf("Expected field 'kubernetes.namespace.keyword', got %v", field)
	}

	// Verify size is 1000 (Logz.io max)
	if terms["size"] != 1000 {
		t.Errorf("Expected aggregation size 1000, got %v", terms["size"])
	}

	// Verify order by _count desc
	order, ok := terms["order"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected order object")
	}

	if order["_count"] != "desc" {
		t.Errorf("Expected order by _count desc, got %+v", order)
	}
}

func TestBuildAggregationQueryWithFilters(t *testing.T) {
	// Test aggregation query with filters
	params := QueryParams{
		Namespace: "prod",
		Level:     "error",
		TimeRange: TimeRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		},
	}

	groupByFields := []string{"kubernetes.pod_name"}

	query := BuildAggregationQuery(params, groupByFields)

	// Marshal to JSON
	queryJSON, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	queryStr := string(queryJSON)

	// Verify filters are present
	if !contains(queryStr, "kubernetes.namespace.keyword") {
		t.Errorf("Expected namespace filter, got:\n%s", queryStr)
	}

	if !contains(queryStr, "level.keyword") {
		t.Errorf("Expected level filter, got:\n%s", queryStr)
	}

	// Verify aggregation on pod_name
	if !contains(queryStr, "kubernetes.pod_name.keyword") {
		t.Errorf("Expected pod_name aggregation, got:\n%s", queryStr)
	}
}

func TestValidateQueryParams_LeadingWildcard(t *testing.T) {
	tests := []struct {
		name        string
		params      QueryParams
		expectError bool
	}{
		{
			name: "leading asterisk wildcard",
			params: QueryParams{
				RegexMatch: "*error",
			},
			expectError: true,
		},
		{
			name: "leading question mark wildcard",
			params: QueryParams{
				RegexMatch: "?error",
			},
			expectError: true,
		},
		{
			name: "suffix wildcard (allowed)",
			params: QueryParams{
				RegexMatch: "error*",
			},
			expectError: false,
		},
		{
			name: "no wildcard",
			params: QueryParams{
				RegexMatch: "(?i)(ERROR|Exception)",
			},
			expectError: false,
		},
		{
			name: "empty regex match",
			params: QueryParams{
				RegexMatch: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQueryParams(tt.params)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for leading wildcard, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateQueryParams_MaxLimit(t *testing.T) {
	tests := []struct {
		name        string
		params      QueryParams
		expectError bool
	}{
		{
			name: "limit within range",
			params: QueryParams{
				Limit: 100,
			},
			expectError: false,
		},
		{
			name: "limit at max (500)",
			params: QueryParams{
				Limit: 500,
			},
			expectError: false,
		},
		{
			name: "limit exceeds max",
			params: QueryParams{
				Limit: 501,
			},
			expectError: true,
		},
		{
			name: "limit zero (default will be used)",
			params: QueryParams{
				Limit: 0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQueryParams(tt.params)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for limit validation, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
