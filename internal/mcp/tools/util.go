package tools

import (
	"fmt"
	"time"
)

// FormatTimestamp converts a Unix timestamp (seconds) to RFC3339 format
func FormatTimestamp(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

// TruncateList limits a string slice to maxItems and adds a truncation indicator if needed
func TruncateList(list []string, maxItems int) []string {
	if len(list) <= maxItems {
		return list
	}

	truncated := make([]string, maxItems+1)
	copy(truncated, list[:maxItems])
	truncated[maxItems] = fmt.Sprintf("(+%d more)", len(list)-maxItems)
	return truncated
}

// ApplyDefaultLimit returns the provided limit, or defaultLimit if limit is 0
// Caps the limit at maxLimit to prevent excessive responses
func ApplyDefaultLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// TruncateMessage truncates a message keeping prefix and suffix with an omission marker.
// If the message is shorter than prefixLen + suffixLen + len(omitMarker), it is returned as-is.
func TruncateMessage(msg string, prefixLen, suffixLen int) string {
	const omitMarker = "[..omitted..]"
	minLen := prefixLen + suffixLen + len(omitMarker)

	if len(msg) <= minLen {
		return msg
	}

	return msg[:prefixLen] + omitMarker + msg[len(msg)-suffixLen:]
}
