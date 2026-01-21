package victorialogs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// LogsTool provides raw log viewing for narrow scope queries
type LogsTool struct {
	ctx ToolContext
}

// LogsParams defines input parameters for logs tool
type LogsParams struct {
	TimeRangeParams
	Namespace string `json:"namespace"`           // Required: namespace to query
	Limit     int    `json:"limit,omitempty"`     // Optional: max logs to return (default 100, max 500)
	Level     string `json:"level,omitempty"`     // Optional: filter by log level
	Pod       string `json:"pod,omitempty"`       // Optional: filter by pod name
	Container string `json:"container,omitempty"` // Optional: filter by container name
}

// LogsResponse returns raw logs
type LogsResponse struct {
	TimeRange string     `json:"time_range"`
	Namespace string     `json:"namespace"`
	Logs      []LogEntry `json:"logs"`      // Raw log entries
	Count     int        `json:"count"`     // Number of logs returned
	Truncated bool       `json:"truncated"` // True if result set was truncated
}

// Execute runs the logs tool
func (t *LogsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	// Parse parameters
	var params LogsParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate required namespace
	if params.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	// Enforce limits (prevent context overflow for AI assistants)
	const MaxLimit = 500
	const DefaultLimit = 100

	if params.Limit == 0 {
		params.Limit = DefaultLimit
	}
	if params.Limit > MaxLimit {
		params.Limit = MaxLimit
	}

	// Parse time range with defaults
	timeRange := parseTimeRange(params.TimeRangeParams)

	// Query raw logs
	queryParams := QueryParams{
		TimeRange: timeRange,
		Namespace: params.Namespace,
		Level:     params.Level,
		Pod:       params.Pod,
		Container: params.Container,
		Limit:     params.Limit + 1, // Fetch one extra to detect truncation
	}

	result, err := t.ctx.Client.QueryLogs(ctx, queryParams)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Check truncation
	truncated := len(result.Logs) > params.Limit
	logs := result.Logs
	if truncated {
		logs = logs[:params.Limit] // Trim to requested limit
	}

	return &LogsResponse{
		TimeRange: fmt.Sprintf("%s to %s", timeRange.Start.Format(time.RFC3339), timeRange.End.Format(time.RFC3339)),
		Namespace: params.Namespace,
		Logs:      logs,
		Count:     len(logs),
		Truncated: truncated,
	}, nil
}
