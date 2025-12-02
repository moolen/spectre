package tools

import "time"

// FormatTimestamp converts a Unix timestamp (seconds) to RFC3339 format
func FormatTimestamp(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}
