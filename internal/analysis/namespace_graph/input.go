package namespacegraph

import (
	"fmt"
	"time"
)

// TimestampBucketSize is the bucket size for timestamp normalization.
// All timestamps near "now" are rounded down to the nearest 30-second boundary.
const TimestampBucketSize = 30 * time.Second

// PrepareAnalyzeInput applies namespace-graph defaults, validation, and timestamp normalization.
func PrepareAnalyzeInput(input AnalyzeInput, now time.Time) (AnalyzeInput, error) {
	if input.Namespace == "" {
		return AnalyzeInput{}, fmt.Errorf("namespace cannot be empty")
	}
	if len(input.Namespace) > 63 {
		return AnalyzeInput{}, fmt.Errorf("namespace must be 63 characters or less")
	}
	if input.Timestamp <= 0 {
		return AnalyzeInput{}, fmt.Errorf("timestamp must be positive")
	}
	if input.Timestamp > now.UnixNano()+int64(time.Hour) {
		return AnalyzeInput{}, fmt.Errorf("timestamp cannot be more than 1 hour in the future")
	}

	input.Timestamp = normalizeTimestamp(input.Timestamp, now.UnixNano())

	if input.Limit <= 0 {
		input.Limit = DefaultLimit
	}
	if input.Limit > MaxLimit {
		input.Limit = MaxLimit
	}
	if input.MaxDepth <= 0 {
		input.MaxDepth = DefaultMaxDepth
	}
	if input.MaxDepth > MaxMaxDepth {
		input.MaxDepth = MaxMaxDepth
	}
	if input.Lookback <= 0 {
		input.Lookback = DefaultLookback
	}
	if input.Lookback > MaxLookback {
		input.Lookback = MaxLookback
	}

	return input, nil
}

func normalizeTimestamp(ts, now int64) int64 {
	delta := now - ts
	if delta < 0 {
		delta = -delta
	}
	if delta <= int64(TimestampBucketSize) {
		bucketNs := int64(TimestampBucketSize)
		return (ts / bucketNs) * bucketNs
	}
	return ts
}
