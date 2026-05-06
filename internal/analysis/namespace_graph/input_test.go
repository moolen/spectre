package namespacegraph

import (
	"testing"
	"time"
)

func TestPrepareAnalyzeInput_AppliesDefaultsAndBucketsNearNow(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 45, 0, time.UTC)
	input := AnalyzeInput{
		Namespace: "default",
		Timestamp: time.Date(2026, 1, 1, 12, 0, 44, 0, time.UTC).UnixNano(),
	}

	got, err := PrepareAnalyzeInput(input, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wantBucketed := (input.Timestamp / int64(TimestampBucketSize)) * int64(TimestampBucketSize)
	if got.Timestamp != wantBucketed {
		t.Fatalf("expected bucketed timestamp %d, got %d", wantBucketed, got.Timestamp)
	}
	if got.Limit != DefaultLimit {
		t.Fatalf("expected default limit %d, got %d", DefaultLimit, got.Limit)
	}
	if got.MaxDepth != DefaultMaxDepth {
		t.Fatalf("expected default max depth %d, got %d", DefaultMaxDepth, got.MaxDepth)
	}
	if got.Lookback != DefaultLookback {
		t.Fatalf("expected default lookback %v, got %v", DefaultLookback, got.Lookback)
	}
}

func TestPrepareAnalyzeInput_PreservesHistoricalTimestamp(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	input := AnalyzeInput{
		Namespace: "default",
		Timestamp: time.Date(2026, 1, 1, 19, 47, 59, 669238289, time.UTC).UnixNano(),
	}

	got, err := PrepareAnalyzeInput(input, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Timestamp != input.Timestamp {
		t.Fatalf("expected historical timestamp %d, got %d", input.Timestamp, got.Timestamp)
	}
}

func TestPrepareAnalyzeInput_RejectsInvalidNamespaceAndFutureTimestamp(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	if _, err := PrepareAnalyzeInput(AnalyzeInput{Timestamp: now.UnixNano()}, now); err == nil {
		t.Fatal("expected error for empty namespace")
	}

	if _, err := PrepareAnalyzeInput(AnalyzeInput{
		Namespace: "default",
		Timestamp: now.Add(2 * time.Hour).UnixNano(),
	}, now); err == nil {
		t.Fatal("expected error for far future timestamp")
	}
}

func TestPrepareAnalyzeInput_ClampsLimitDepthAndLookback(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	input := AnalyzeInput{
		Namespace: "default",
		Timestamp: now.UnixNano(),
		Limit:     MaxLimit + 100,
		MaxDepth:  MaxMaxDepth + 10,
		Lookback:  MaxLookback + time.Hour,
	}

	got, err := PrepareAnalyzeInput(input, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Limit != MaxLimit {
		t.Fatalf("expected clamped limit %d, got %d", MaxLimit, got.Limit)
	}
	if got.MaxDepth != MaxMaxDepth {
		t.Fatalf("expected clamped max depth %d, got %d", MaxMaxDepth, got.MaxDepth)
	}
	if got.Lookback != MaxLookback {
		t.Fatalf("expected clamped lookback %v, got %v", MaxLookback, got.Lookback)
	}
}
