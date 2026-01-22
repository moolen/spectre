package grafana

import (
	"fmt"
	"regexp"

	"github.com/prometheus/prometheus/promql/parser"
)

// QueryExtraction holds semantic components extracted from a PromQL query.
// Used for building Dashboard→Query→Metric relationships in the graph.
type QueryExtraction struct {
	// MetricNames contains all metric names extracted from VectorSelector nodes.
	// Multiple metrics may appear in complex queries (e.g., binary operations).
	MetricNames []string

	// LabelSelectors maps label names to their matcher values (equality only).
	// Example: {job="api", handler="/health"} → {"job": "api", "handler": "/health"}
	LabelSelectors map[string]string

	// Aggregations contains all aggregation functions and calls extracted from the query.
	// Example: sum(rate(metric[5m])) → ["sum", "rate"]
	Aggregations []string

	// HasVariables indicates if the query contains Grafana template variable syntax.
	// Examples: $var, ${var}, ${var:csv}, [[var]]
	HasVariables bool
}

// variablePatterns define Grafana template variable syntax patterns.
// Reference: https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/variable-syntax/
var variablePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$\w+`),          // $var
	regexp.MustCompile(`\$\{\w+\}`),      // ${var}
	regexp.MustCompile(`\$\{\w+:\w+\}`),  // ${var:format}
	regexp.MustCompile(`\[\[\w+\]\]`),    // [[var]] (deprecated Grafana 7.0+)
}

// hasVariableSyntax checks if a string contains Grafana variable syntax.
func hasVariableSyntax(str string) bool {
	for _, pattern := range variablePatterns {
		if pattern.MatchString(str) {
			return true
		}
	}
	return false
}

// ExtractFromPromQL parses a PromQL query using the official Prometheus parser
// and extracts semantic components (metric names, labels, aggregations).
//
// Uses AST-based traversal via parser.Inspect for reliable extraction.
// Returns nil extraction with error for unparseable queries (graceful handling).
//
// Variable detection: Grafana variable syntax ($var, ${var}, [[var]]) is detected
// but not interpolated - queries with variables have HasVariables=true flag set.
// If the query contains variable syntax that makes it unparseable by the Prometheus
// parser, the function detects the variables and returns a basic extraction.
func ExtractFromPromQL(queryStr string) (*QueryExtraction, error) {
	// Initialize extraction struct with empty collections
	extraction := &QueryExtraction{
		MetricNames:    make([]string, 0),
		LabelSelectors: make(map[string]string),
		Aggregations:   make([]string, 0),
		HasVariables:   false,
	}

	// Check for variable syntax in the entire query string
	// This is done first because variables may make the query unparseable
	if hasVariableSyntax(queryStr) {
		extraction.HasVariables = true
	}

	// Parse PromQL expression into AST
	expr, err := parser.ParseExpr(queryStr)
	if err != nil {
		// If parsing fails and we detected variables, return partial extraction
		// This is expected for queries with Grafana variable syntax
		if extraction.HasVariables {
			return extraction, nil
		}
		// Graceful error handling: return nil extraction with context
		return nil, fmt.Errorf("failed to parse PromQL: %w", err)
	}

	// Walk AST in depth-first order to extract semantic components
	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if node == nil {
			return nil
		}

		switch n := node.(type) {
		case *parser.VectorSelector:
			// Extract metric name from VectorSelector
			// CRITICAL: Check if Name is non-empty (handles label-only selectors like {job="api"})
			if n.Name != "" {
				// Check for variable syntax in metric name
				if hasVariableSyntax(n.Name) {
					extraction.HasVariables = true
				} else {
					// Only add concrete metric names (no variables)
					extraction.MetricNames = append(extraction.MetricNames, n.Name)
				}
			}

			// Extract label matchers (handle equality matchers only)
			for _, matcher := range n.LabelMatchers {
				// Skip the __name__ label (it's the metric name)
				if matcher.Name == "__name__" {
					continue
				}

				// Check for variable syntax in label values
				if hasVariableSyntax(matcher.Value) {
					extraction.HasVariables = true
				}

				// Store equality matchers in map
				// TODO: Handle regex matchers (=~, !~) if needed downstream
				extraction.LabelSelectors[matcher.Name] = matcher.Value
			}

		case *parser.AggregateExpr:
			// Extract aggregation operator (sum, avg, min, max, count, etc.)
			aggregation := n.Op.String()
			extraction.Aggregations = append(extraction.Aggregations, aggregation)

		case *parser.Call:
			// Extract function calls (rate, increase, irate, delta, etc.)
			extraction.Aggregations = append(extraction.Aggregations, n.Func.Name)
		}

		return nil
	})

	return extraction, nil
}
