package victorialogs

import (
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ToolContext provides shared context for tool execution
type ToolContext struct {
	Client   *Client
	Logger   *logging.Logger
	Instance string // Integration instance name (e.g., "prod", "staging")
}

// TimeRangeParams represents time range input for tools
type TimeRangeParams struct {
	StartTime int64 `json:"start_time,omitempty"` // Unix seconds or milliseconds
	EndTime   int64 `json:"end_time,omitempty"`   // Unix seconds or milliseconds
}

// parseTimeRange converts TimeRangeParams to TimeRange with defaults
// Default: last 1 hour if not specified
// Minimum: 15 minutes (enforced by BuildLogsQLQuery via VLOG-03)
func parseTimeRange(params TimeRangeParams) TimeRange {
	now := time.Now()

	// Default: last 1 hour
	if params.StartTime == 0 && params.EndTime == 0 {
		return TimeRange{
			Start: now.Add(-1 * time.Hour),
			End:   now,
		}
	}

	// Parse start time
	start := now.Add(-1 * time.Hour) // Default if only end provided
	if params.StartTime != 0 {
		start = parseTimestamp(params.StartTime)
	}

	// Parse end time
	end := now // Default if only start provided
	if params.EndTime != 0 {
		end = parseTimestamp(params.EndTime)
	}

	return TimeRange{Start: start, End: end}
}

// parseTimestamp converts Unix timestamp (seconds or milliseconds) to time.Time
func parseTimestamp(ts int64) time.Time {
	// Heuristic: if > 10^10, it's milliseconds, else seconds
	if ts > 10000000000 {
		return time.Unix(0, ts*int64(time.Millisecond))
	}
	return time.Unix(ts, 0)
}
