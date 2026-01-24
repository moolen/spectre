package grafana

import (
	"testing"
)

func TestExtractFromPromQL_SimpleMetric(t *testing.T) {
	query := `http_requests_total`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric name extracted
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}
	if extraction.MetricNames[0] != "http_requests_total" {
		t.Errorf("expected metric 'http_requests_total', got '%s'", extraction.MetricNames[0])
	}

	// Verify no aggregations
	if len(extraction.Aggregations) != 0 {
		t.Errorf("expected 0 aggregations, got %d", len(extraction.Aggregations))
	}

	// Verify no variables
	if extraction.HasVariables {
		t.Error("expected HasVariables=false")
	}
}

func TestExtractFromPromQL_WithAggregation(t *testing.T) {
	query := `sum(rate(http_requests_total[5m])) by (status)`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric name extracted
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}
	if extraction.MetricNames[0] != "http_requests_total" {
		t.Errorf("expected metric 'http_requests_total', got '%s'", extraction.MetricNames[0])
	}

	// Verify aggregations extracted
	if len(extraction.Aggregations) != 2 {
		t.Fatalf("expected 2 aggregations, got %d", len(extraction.Aggregations))
	}

	// Check that both sum and rate are present (order may vary)
	hasSum := false
	hasRate := false
	for _, agg := range extraction.Aggregations {
		if agg == "sum" {
			hasSum = true
		}
		if agg == "rate" {
			hasRate = true
		}
	}
	if !hasSum {
		t.Error("expected 'sum' aggregation")
	}
	if !hasRate {
		t.Error("expected 'rate' aggregation")
	}
}

func TestExtractFromPromQL_WithLabelSelectors(t *testing.T) {
	query := `http_requests_total{job="api", handler="/health"}`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric name extracted
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}

	// Verify label selectors extracted
	if len(extraction.LabelSelectors) != 2 {
		t.Fatalf("expected 2 label selectors, got %d", len(extraction.LabelSelectors))
	}

	if extraction.LabelSelectors["job"] != "api" {
		t.Errorf("expected job='api', got '%s'", extraction.LabelSelectors["job"])
	}
	if extraction.LabelSelectors["handler"] != "/health" {
		t.Errorf("expected handler='/health', got '%s'", extraction.LabelSelectors["handler"])
	}
}

func TestExtractFromPromQL_LabelOnlySelector(t *testing.T) {
	// Tests Pitfall 1: VectorSelector without metric name
	query := `{job="api", handler="/health"}`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no metric names (empty name)
	if len(extraction.MetricNames) != 0 {
		t.Errorf("expected 0 metrics for label-only selector, got %d", len(extraction.MetricNames))
	}

	// Verify label selectors still extracted
	if len(extraction.LabelSelectors) != 2 {
		t.Fatalf("expected 2 label selectors, got %d", len(extraction.LabelSelectors))
	}

	if extraction.LabelSelectors["job"] != "api" {
		t.Errorf("expected job='api', got '%s'", extraction.LabelSelectors["job"])
	}
	if extraction.LabelSelectors["handler"] != "/health" {
		t.Errorf("expected handler='/health', got '%s'", extraction.LabelSelectors["handler"])
	}
}

func TestExtractFromPromQL_VariableSyntax(t *testing.T) {
	// Test all 4 Grafana variable syntax patterns
	// These queries are unparseable by Prometheus parser but should gracefully return partial extraction
	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "dollar sign syntax",
			query: `http_requests_$service_total`,
		},
		{
			name:  "curly braces syntax",
			query: `http_requests_${service}_total`,
		},
		{
			name:  "curly braces with format",
			query: `http_requests_${service:csv}_total`,
		},
		{
			name:  "deprecated bracket syntax",
			query: `http_requests_[[service]]_total`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			extraction, err := ExtractFromPromQL(tc.query)
			// No error expected - variable syntax is detected and gracefully handled
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify HasVariables flag set
			if !extraction.HasVariables {
				t.Error("expected HasVariables=true for query with variable syntax")
			}

			// Verify metric name NOT added (unparseable due to variable)
			if len(extraction.MetricNames) != 0 {
				t.Errorf("expected 0 metric names for variable-containing query, got %d", len(extraction.MetricNames))
			}
		})
	}
}

func TestExtractFromPromQL_NestedAggregations(t *testing.T) {
	query := `avg(sum(rate(http_requests_total[5m])) by (status))`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric name extracted
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}

	// Verify all 3 aggregations extracted
	if len(extraction.Aggregations) != 3 {
		t.Fatalf("expected 3 aggregations, got %d", len(extraction.Aggregations))
	}

	// Check all aggregations present (order may vary based on traversal)
	hasAvg := false
	hasSum := false
	hasRate := false
	for _, agg := range extraction.Aggregations {
		if agg == "avg" {
			hasAvg = true
		}
		if agg == "sum" {
			hasSum = true
		}
		if agg == "rate" {
			hasRate = true
		}
	}

	if !hasAvg {
		t.Error("expected 'avg' aggregation")
	}
	if !hasSum {
		t.Error("expected 'sum' aggregation")
	}
	if !hasRate {
		t.Error("expected 'rate' aggregation")
	}
}

func TestExtractFromPromQL_InvalidQuery(t *testing.T) {
	// Tests Pitfall 2: graceful error handling
	query := `sum(rate(http_requests_total[5m]) by (status)` // Missing closing parenthesis
	extraction, err := ExtractFromPromQL(query)

	// Verify error returned
	if err == nil {
		t.Fatal("expected error for malformed PromQL, got nil")
	}

	// Verify nil extraction
	if extraction != nil {
		t.Error("expected nil extraction for parse error")
	}
}

func TestExtractFromPromQL_EmptyQuery(t *testing.T) {
	query := ``
	extraction, err := ExtractFromPromQL(query)

	// Verify error returned for empty query
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}

	// Verify nil extraction
	if extraction != nil {
		t.Error("expected nil extraction for empty query")
	}
}

func TestExtractFromPromQL_ComplexQuery(t *testing.T) {
	// Real-world Grafana query with multiple metrics in binary expression
	query := `(sum(container_memory_usage_bytes{namespace="$namespace"}) / sum(container_spec_memory_limit_bytes{namespace="$namespace"})) * 100`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both metrics extracted
	if len(extraction.MetricNames) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(extraction.MetricNames))
	}

	// Check both metric names present (order may vary)
	hasUsage := false
	hasLimit := false
	for _, metric := range extraction.MetricNames {
		if metric == "container_memory_usage_bytes" {
			hasUsage = true
		}
		if metric == "container_spec_memory_limit_bytes" {
			hasLimit = true
		}
	}

	if !hasUsage {
		t.Error("expected 'container_memory_usage_bytes' metric")
	}
	if !hasLimit {
		t.Error("expected 'container_spec_memory_limit_bytes' metric")
	}

	// Verify HasVariables flag set (query contains $namespace)
	if !extraction.HasVariables {
		t.Error("expected HasVariables=true for query with $namespace variable")
	}

	// Verify aggregations extracted
	if len(extraction.Aggregations) < 2 {
		t.Errorf("expected at least 2 aggregations (sum), got %d", len(extraction.Aggregations))
	}
}

func TestExtractFromPromQL_MultipleMetricsInBinaryOp(t *testing.T) {
	query := `node_memory_MemTotal_bytes - node_memory_MemFree_bytes`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both metrics extracted
	if len(extraction.MetricNames) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(extraction.MetricNames))
	}

	// Check both metric names present
	hasTotal := false
	hasFree := false
	for _, metric := range extraction.MetricNames {
		if metric == "node_memory_MemTotal_bytes" {
			hasTotal = true
		}
		if metric == "node_memory_MemFree_bytes" {
			hasFree = true
		}
	}

	if !hasTotal {
		t.Error("expected 'node_memory_MemTotal_bytes' metric")
	}
	if !hasFree {
		t.Error("expected 'node_memory_MemFree_bytes' metric")
	}
}

func TestExtractFromPromQL_FunctionsWithoutAggregations(t *testing.T) {
	query := `increase(http_requests_total[5m])`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric extracted
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}

	// Verify increase function extracted
	if len(extraction.Aggregations) != 1 {
		t.Fatalf("expected 1 aggregation (increase), got %d", len(extraction.Aggregations))
	}
	if extraction.Aggregations[0] != "increase" {
		t.Errorf("expected 'increase' aggregation, got '%s'", extraction.Aggregations[0])
	}
}

func TestExtractFromPromQL_MatrixSelector(t *testing.T) {
	query := `rate(http_requests_total[5m])`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric extracted (matrix selector has underlying VectorSelector)
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}
	if extraction.MetricNames[0] != "http_requests_total" {
		t.Errorf("expected metric 'http_requests_total', got '%s'", extraction.MetricNames[0])
	}

	// Verify rate function extracted
	if len(extraction.Aggregations) != 1 {
		t.Fatalf("expected 1 aggregation (rate), got %d", len(extraction.Aggregations))
	}
	if extraction.Aggregations[0] != "rate" {
		t.Errorf("expected 'rate' aggregation, got '%s'", extraction.Aggregations[0])
	}
}

func TestExtractFromPromQL_VariableInLabelSelector(t *testing.T) {
	query := `http_requests_total{namespace="$namespace", pod=~"$pod"}`
	extraction, err := ExtractFromPromQL(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metric extracted
	if len(extraction.MetricNames) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(extraction.MetricNames))
	}

	// Verify HasVariables flag set (label values contain variables)
	if !extraction.HasVariables {
		t.Error("expected HasVariables=true for query with variables in label selectors")
	}

	// Verify label selectors extracted (even with variable values)
	if len(extraction.LabelSelectors) < 1 {
		t.Errorf("expected label selectors to be extracted, got %d", len(extraction.LabelSelectors))
	}
}
