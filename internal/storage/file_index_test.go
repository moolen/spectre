package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

//nolint:goprintffuncname // Interface requires these method names
func (l *testLogger) Info(format string, args ...interface{}) {}

//nolint:goprintffuncname // Interface requires these method names
func (l *testLogger) Debug(format string, args ...interface{}) {}

//nolint:goprintffuncname // Interface requires these method names
func (l *testLogger) Warn(format string, args ...interface{}) {}

func TestFileIndex_AddGetRemove(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}

	index := NewFileIndex(tmpDir, logger)

	// Add a file
	hourStart := time.Date(2024, 12, 13, 14, 0, 0, 0, time.UTC).Unix()
	meta := &FileMetadata{
		FilePath:     filepath.Join(tmpDir, "2024-12-13-14.bin"),
		HourStart:    hourStart,
		HourEnd:      hourStart + 3600,
		TimestampMin: hourStart * 1e9,
		TimestampMax: (hourStart + 3600) * 1e9,
		EventCount:   100,
		FileSize:     1024,
	}

	err := index.AddOrUpdate(meta)
	require.NoError(t, err)

	// Get it back
	retrieved, ok := index.Get(meta.FilePath)
	require.True(t, ok)
	assert.Equal(t, meta.FilePath, retrieved.FilePath)
	assert.Equal(t, meta.HourStart, retrieved.HourStart)
	assert.Equal(t, meta.EventCount, retrieved.EventCount)

	// Count
	assert.Equal(t, 1, index.Count())

	// Remove it
	err = index.Remove(meta.FilePath)
	require.NoError(t, err)

	// Should be gone
	_, ok = index.Get(meta.FilePath)
	assert.False(t, ok)
	assert.Equal(t, 0, index.Count())
}

func TestFileIndex_GetFilesByTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}

	index := NewFileIndex(tmpDir, logger)
	index.SetStrictHours(true)

	// Add files for 3 different hours
	baseTime := time.Date(2024, 12, 13, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		hourStart := baseTime.Add(time.Duration(i) * time.Hour).Unix()
		meta := &FileMetadata{
			FilePath:     filepath.Join(tmpDir, "file-"+string(rune('0'+i))+".bin"),
			HourStart:    hourStart,
			HourEnd:      hourStart + 3600,
			TimestampMin: hourStart * 1e9,
			TimestampMax: (hourStart + 3600) * 1e9,
			EventCount:   100,
		}
		require.NoError(t, index.AddOrUpdate(meta))
	}

	// Query middle hour (should return 1 file)
	hour1Start := baseTime.Add(time.Hour).Unix() * 1e9
	hour1End := hour1Start + (3600 * 1e9)
	files := index.GetFilesByTimeRange(hour1Start, hour1End)
	assert.Equal(t, 1, len(files))

	// Query spanning all hours (should return 3 files)
	hour0Start := baseTime.Unix() * 1e9
	hour2End := baseTime.Add(3*time.Hour).Unix() * 1e9
	files = index.GetFilesByTimeRange(hour0Start, hour2End)
	assert.Equal(t, 3, len(files))

	// Query before all files (should return 0)
	beforeStart := baseTime.Add(-2*time.Hour).Unix() * 1e9
	beforeEnd := baseTime.Add(-1*time.Hour).Unix() * 1e9
	files = index.GetFilesByTimeRange(beforeStart, beforeEnd)
	assert.Equal(t, 0, len(files))
}

func TestFileIndex_GetFileBeforeTime(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}

	index := NewFileIndex(tmpDir, logger)

	// Add files for different hours
	baseTime := time.Date(2024, 12, 13, 10, 0, 0, 0, time.UTC)

	var filePath1, filePath2 string
	for i := 0; i < 3; i++ {
		hourStart := baseTime.Add(time.Duration(i) * time.Hour).Unix()
		filePath := filepath.Join(tmpDir, "file-"+string(rune('0'+i))+".bin")
		if i == 1 {
			filePath1 = filePath
		} else if i == 2 {
			filePath2 = filePath
		}

		meta := &FileMetadata{
			FilePath:     filePath,
			HourStart:    hourStart,
			HourEnd:      hourStart + 3600,
			TimestampMin: hourStart * 1e9,
			TimestampMax: (hourStart + 3600) * 1e9,
		}
		require.NoError(t, index.AddOrUpdate(meta))
	}

	// Query file before hour 2 (should return hour 1)
	hour2Start := baseTime.Add(2*time.Hour).Unix() * 1e9
	file := index.GetFileBeforeTime(hour2Start)
	assert.Equal(t, filePath1, file)

	// Query file before hour 3 (should return hour 2)
	hour3Start := baseTime.Add(3*time.Hour).Unix() * 1e9
	file = index.GetFileBeforeTime(hour3Start)
	assert.Equal(t, filePath2, file)

	// Query file before hour 0 (should return nothing)
	hour0Start := baseTime.Unix() * 1e9
	file = index.GetFileBeforeTime(hour0Start)
	assert.Equal(t, "", file)
}

func TestFileIndex_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}

	// Create index and add files
	index1 := NewFileIndex(tmpDir, logger)
	index1.autoSave = true

	hourStart := time.Date(2024, 12, 13, 14, 0, 0, 0, time.UTC).Unix()
	meta := &FileMetadata{
		FilePath:     filepath.Join(tmpDir, "2024-12-13-14.bin"),
		HourStart:    hourStart,
		HourEnd:      hourStart + 3600,
		TimestampMin: hourStart * 1e9,
		TimestampMax: (hourStart + 3600) * 1e9,
		EventCount:   100,
		FileSize:     1024,
	}

	err := index1.AddOrUpdate(meta)
	require.NoError(t, err)

	// Create new index and load
	index2 := NewFileIndex(tmpDir, logger)
	err = index2.Load()
	require.NoError(t, err)

	// Should have the file
	assert.Equal(t, 1, index2.Count())
	retrieved, ok := index2.Get(meta.FilePath)
	require.True(t, ok)
	assert.Equal(t, meta.HourStart, retrieved.HourStart)
	assert.Equal(t, meta.EventCount, retrieved.EventCount)
}

func TestFileIndex_StrictHours(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}

	index := NewFileIndex(tmpDir, logger)

	hourStart := time.Date(2024, 12, 13, 14, 0, 0, 0, time.UTC).Unix()

	// Add file with events that extend beyond hour boundary (legacy mode)
	meta := &FileMetadata{
		FilePath:     filepath.Join(tmpDir, "2024-12-13-14.bin"),
		HourStart:    hourStart,
		HourEnd:      hourStart + 3600,
		TimestampMin: (hourStart - 100) * 1e9,  // 100s before hour
		TimestampMax: (hourStart + 3700) * 1e9, // 100s after hour end
		EventCount:   100,
	}
	require.NoError(t, index.AddOrUpdate(meta))

	// Test strict mode - should use hour boundaries only
	index.SetStrictHours(true)
	hourStartNs := hourStart * 1e9
	hourEndNs := hourStartNs + (3600 * 1e9)

	// Query within hour - should match
	files := index.GetFilesByTimeRange(hourStartNs, hourEndNs)
	assert.Equal(t, 1, len(files))

	// Query before hour - should NOT match (even though events extend before)
	beforeHourStart := (hourStart - 200) * 1e9
	beforeHourEnd := (hourStart - 50) * 1e9
	files = index.GetFilesByTimeRange(beforeHourStart, beforeHourEnd)
	assert.Equal(t, 0, len(files))

	// Test non-strict mode - should use actual event timestamps
	index.SetStrictHours(false)

	// Query before hour - should NOW match because events extend before
	files = index.GetFilesByTimeRange(beforeHourStart, beforeHourEnd)
	assert.Equal(t, 1, len(files))
}
