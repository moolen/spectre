package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeNamespaceGraphTimestamp_BucketsNearNow(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 45, 0, time.UTC).UnixNano()
	input := time.Date(2026, 1, 1, 12, 0, 44, 0, time.UTC).UnixNano()

	require.Equal(t, bucketTimestamp(input), normalizeNamespaceGraphTimestamp(input, now))
}

func TestNormalizeNamespaceGraphTimestamp_PreservesHistoricalTimestamp(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC).UnixNano()
	input := time.Date(2026, 1, 1, 19, 47, 59, 669238289, time.UTC).UnixNano()

	require.Equal(t, input, normalizeNamespaceGraphTimestamp(input, now))
}
