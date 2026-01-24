package victorialogs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// OverviewTool provides global overview of log volume and severity by namespace
type OverviewTool struct {
	ctx ToolContext
}

// OverviewParams defines input parameters for overview tool
type OverviewParams struct {
	TimeRangeParams
	Namespace string `json:"namespace,omitempty"` // Optional: filter to specific namespace
}

// OverviewResponse returns namespace-level severity counts
type OverviewResponse struct {
	TimeRange  string              `json:"time_range"`  // Human-readable time range
	Namespaces []NamespaceSeverity `json:"namespaces"`  // Counts by namespace, sorted by total desc
	TotalLogs  int                 `json:"total_logs"`  // Total log count across all namespaces
}

// NamespaceSeverity holds severity counts for a namespace
type NamespaceSeverity struct {
	Namespace string `json:"namespace"`
	Errors    int    `json:"errors"`
	Warnings  int    `json:"warnings"`
	Other     int    `json:"other"` // Non-error/warning logs
	Total     int    `json:"total"` // Sum of all severities
}

// Execute runs the overview tool
func (t *OverviewTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	// Parse parameters
	var params OverviewParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Parse time range with defaults
	timeRange := parseTimeRange(params.TimeRangeParams)

	// Build base query parameters
	baseQuery := QueryParams{
		TimeRange: timeRange,
		Namespace: params.Namespace,
	}

	// Execute all 3 queries in parallel to reduce total latency
	// This reduces time from ~16s (sequential) to ~10s (parallel)
	type queryResult struct {
		name   string
		result *AggregationResponse
		err    error
	}

	resultCh := make(chan queryResult, 3)

	// Query 1: Total logs per namespace
	go func() {
		result, err := t.ctx.Client.QueryAggregation(ctx, baseQuery, []string{"namespace"})
		resultCh <- queryResult{name: "total", result: result, err: err}
	}()

	// Query 2: Error logs
	go func() {
		errorQuery := baseQuery
		errorQuery.RegexMatch = GetErrorPattern()
		result, err := t.ctx.Client.QueryAggregation(ctx, errorQuery, []string{"namespace"})
		resultCh <- queryResult{name: "error", result: result, err: err}
	}()

	// Query 3: Warning logs
	go func() {
		warnQuery := baseQuery
		warnQuery.RegexMatch = GetWarningPattern()
		result, err := t.ctx.Client.QueryAggregation(ctx, warnQuery, []string{"namespace"})
		resultCh <- queryResult{name: "warn", result: result, err: err}
	}()

	// Collect results
	var totalResult, errorResult, warnResult *AggregationResponse
	for i := 0; i < 3; i++ {
		r := <-resultCh
		switch r.name {
		case "total":
			if r.err != nil {
				return nil, fmt.Errorf("total query failed: %w", r.err)
			}
			totalResult = r.result
		case "error":
			if r.err != nil {
				t.ctx.Logger.Warn("Error query failed: %v", r.err)
				errorResult = &AggregationResponse{Groups: []AggregationGroup{}}
			} else {
				errorResult = r.result
			}
		case "warn":
			if r.err != nil {
				t.ctx.Logger.Warn("Warning query failed: %v", r.err)
				warnResult = &AggregationResponse{Groups: []AggregationGroup{}}
			} else {
				warnResult = r.result
			}
		}
	}

	// Aggregate results by namespace
	namespaceMap := make(map[string]*NamespaceSeverity)

	// Process total counts
	for _, group := range totalResult.Groups {
		ns := group.Value
		if ns == "" {
			ns = "(no namespace)"
		}
		namespaceMap[ns] = &NamespaceSeverity{
			Namespace: ns,
			Total:     group.Count,
		}
	}

	// Process error counts
	for _, group := range errorResult.Groups {
		ns := group.Value
		if ns == "" {
			ns = "(no namespace)"
		}
		if _, exists := namespaceMap[ns]; !exists {
			namespaceMap[ns] = &NamespaceSeverity{Namespace: ns}
		}
		namespaceMap[ns].Errors = group.Count
	}

	// Process warning counts
	for _, group := range warnResult.Groups {
		ns := group.Value
		if ns == "" {
			ns = "(no namespace)"
		}
		if _, exists := namespaceMap[ns]; !exists {
			namespaceMap[ns] = &NamespaceSeverity{Namespace: ns}
		}
		namespaceMap[ns].Warnings = group.Count
	}

	// Calculate "other" (total - errors - warnings)
	for _, ns := range namespaceMap {
		ns.Other = ns.Total - ns.Errors - ns.Warnings
		if ns.Other < 0 {
			ns.Other = 0 // Overlap possible if logs have multiple levels
		}
	}

	// Convert to slice and sort by total descending (most logs first)
	namespaces := make([]NamespaceSeverity, 0, len(namespaceMap))
	totalLogs := 0
	for _, ns := range namespaceMap {
		namespaces = append(namespaces, *ns)
		totalLogs += ns.Total
	}

	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Total > namespaces[j].Total
	})

	// Build response
	return &OverviewResponse{
		TimeRange:  fmt.Sprintf("%s to %s", timeRange.Start.Format(time.RFC3339), timeRange.End.Format(time.RFC3339)),
		Namespaces: namespaces,
		TotalLogs:  totalLogs,
	}, nil
}
