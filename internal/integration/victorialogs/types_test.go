package victorialogs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeRange_ValidateMinimumDuration(t *testing.T) {
	tests := []struct {
		name        string
		timeRange   TimeRange
		minDuration time.Duration
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid range - exactly 15 minutes",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC),
			},
			minDuration: 15 * time.Minute,
			expectError: false,
		},
		{
			name: "valid range - 30 minutes",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC),
			},
			minDuration: 15 * time.Minute,
			expectError: false,
		},
		{
			name: "valid range - 1 hour",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			},
			minDuration: 15 * time.Minute,
			expectError: false,
		},
		{
			name: "invalid range - 14 minutes",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 12, 14, 0, 0, time.UTC),
			},
			minDuration: 15 * time.Minute,
			expectError: true,
			errorMsg:    "time range duration 14m0s is below minimum 15m0s",
		},
		{
			name: "invalid range - 1 minute",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC),
			},
			minDuration: 15 * time.Minute,
			expectError: true,
			errorMsg:    "time range duration 1m0s is below minimum 15m0s",
		},
		{
			name: "invalid range - 1 second",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
			},
			minDuration: 15 * time.Minute,
			expectError: true,
			errorMsg:    "time range duration 1s is below minimum 15m0s",
		},
		{
			name:        "zero time range - no validation",
			timeRange:   TimeRange{},
			minDuration: 15 * time.Minute,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.timeRange.ValidateMinimumDuration(tt.minDuration)

			if tt.expectError {
				require.Error(t, err, "Expected validation error but got none")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message mismatch")
			} else {
				assert.NoError(t, err, "Expected no validation error")
			}
		})
	}
}

func TestTimeRange_Duration(t *testing.T) {
	tests := []struct {
		name      string
		timeRange TimeRange
		expected  time.Duration
	}{
		{
			name: "15 minutes",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC),
			},
			expected: 15 * time.Minute,
		},
		{
			name: "1 hour",
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			},
			expected: 1 * time.Hour,
		},
		{
			name:      "zero time range",
			timeRange: TimeRange{},
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := tt.timeRange.Duration()
			assert.Equal(t, tt.expected, duration)
		})
	}
}

func TestDefaultTimeRange(t *testing.T) {
	tr := DefaultTimeRange()

	// Verify it returns approximately 1 hour duration
	duration := tr.Duration()
	assert.InDelta(t, float64(time.Hour), float64(duration), float64(time.Second),
		"DefaultTimeRange should return approximately 1 hour")

	// Verify End is after Start
	assert.True(t, tr.End.After(tr.Start), "End should be after Start")

	// Verify time range is recent (within last 2 seconds)
	assert.WithinDuration(t, time.Now(), tr.End, 2*time.Second,
		"End should be close to current time")
}
