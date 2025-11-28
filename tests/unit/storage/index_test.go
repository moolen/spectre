package storage

import (
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

// TestNewIndexManager tests index manager creation
func TestNewIndexManager(t *testing.T) {
	im := storage.NewIndexManager()
	if im == nil {
		t.Error("NewIndexManager returned nil")
	}
}

// TestAddEntry tests adding an index entry
func TestAddEntry(t *testing.T) {
	im := storage.NewIndexManager()

	im.AddEntry(1, 1000, 100)

	if im.GetSegmentCount() != 1 {
		t.Errorf("Expected 1 segment, got %d", im.GetSegmentCount())
	}
}

// TestAddMultipleEntries tests adding multiple entries
func TestAddMultipleEntries(t *testing.T) {
	im := storage.NewIndexManager()

	entries := []struct {
		segmentID int32
		timestamp int64
		offset    int64
	}{
		{1, 1000, 100},
		{2, 2000, 200},
		{3, 3000, 300},
		{4, 4000, 400},
	}

	for _, e := range entries {
		im.AddEntry(e.segmentID, e.timestamp, e.offset)
	}

	if im.GetSegmentCount() != len(entries) {
		t.Errorf("Expected %d segments, got %d", len(entries), im.GetSegmentCount())
	}
}

// TestFindSegmentsInTimeRange tests finding segments in time range
func TestFindSegmentsInTimeRange(t *testing.T) {
	im := storage.NewIndexManager()

	// Add entries at specific timestamps
	im.AddEntry(1, 1000, 100)
	im.AddEntry(2, 2000, 200)
	im.AddEntry(3, 3000, 300)
	im.AddEntry(4, 4000, 400)
	im.AddEntry(5, 5000, 500)

	testCases := []struct {
		name          string
		startTime     int64
		endTime       int64
		expectedCount int
	}{
		{"full range", 0, 6000, 5},
		{"partial range", 2000, 4000, 4}, // entries 1, 2, 3, 4 (includes 1 < 2000, and 2, 3, 4 in range)
		{"single segment", 1000, 1000, 1},
		{"before all", 0, 999, 0},
		{"after all", 6000, 7000, 5},    // includes all (conservative - all < 6000)
		{"narrow range", 2500, 3500, 3}, // entries 1, 2, 3 (1 < 2500, 2 < 2500, 3 in range)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := im.FindSegmentsInTimeRange(tc.startTime, tc.endTime)
			if len(result) != tc.expectedCount {
				t.Errorf("Expected %d segments, got %d", tc.expectedCount, len(result))
			}
		})
	}
}

// TestGetSegmentOffset tests getting segment offset
func TestGetSegmentOffset(t *testing.T) {
	im := storage.NewIndexManager()

	im.AddEntry(1, 1000, 500)
	im.AddEntry(2, 2000, 1000)

	offset, err := im.GetSegmentOffset(1)
	if err != nil {
		t.Fatalf("GetSegmentOffset failed: %v", err)
	}

	if offset != 500 {
		t.Errorf("Expected offset 500, got %d", offset)
	}
}

// TestGetSegmentOffsetNotFound tests getting non-existent segment offset
func TestGetSegmentOffsetNotFound(t *testing.T) {
	im := storage.NewIndexManager()

	im.AddEntry(1, 1000, 500)

	_, err := im.GetSegmentOffset(999)
	if err == nil {
		t.Error("Expected error for non-existent segment")
	}
}

// TestBuildSparseIndex tests building sparse index
func TestBuildSparseIndex(t *testing.T) {
	im := storage.NewIndexManager()

	im.AddEntry(1, 1000, 100)
	im.AddEntry(2, 2000, 200)
	im.AddEntry(3, 3000, 300)

	sparseIndex := im.BuildSparseIndex()

	if sparseIndex.TotalSegments != 3 {
		t.Errorf("Expected 3 total segments, got %d", sparseIndex.TotalSegments)
	}

	if len(sparseIndex.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(sparseIndex.Entries))
	}
}

// TestGetEntriesInRange tests getting entries in range
func TestGetEntriesInRange(t *testing.T) {
	im := storage.NewIndexManager()

	im.AddEntry(1, 1000, 100)
	im.AddEntry(2, 2000, 200)
	im.AddEntry(3, 3000, 300)
	im.AddEntry(4, 4000, 400)

	entries := im.GetEntriesInRange(2000, 3500)

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry.Timestamp < 2000 || entry.Timestamp > 3500 {
			t.Errorf("Entry timestamp %d outside range", entry.Timestamp)
		}
	}
}

// TestClear tests clearing the index
func TestClear(t *testing.T) {
	im := storage.NewIndexManager()

	im.AddEntry(1, 1000, 100)
	im.AddEntry(2, 2000, 200)

	if im.GetSegmentCount() != 2 {
		t.Errorf("Expected 2 segments before clear")
	}

	im.Clear()

	if im.GetSegmentCount() != 0 {
		t.Errorf("Expected 0 segments after clear, got %d", im.GetSegmentCount())
	}
}

// TestGetAllEntries tests getting all entries
func TestGetAllEntries(t *testing.T) {
	im := storage.NewIndexManager()

	expectedEntries := []struct {
		segmentID int32
		timestamp int64
		offset    int64
	}{
		{1, 1000, 100},
		{2, 2000, 200},
		{3, 3000, 300},
	}

	for _, e := range expectedEntries {
		im.AddEntry(e.segmentID, e.timestamp, e.offset)
	}

	allEntries := im.GetAllEntries()

	if len(allEntries) != len(expectedEntries) {
		t.Errorf("Expected %d entries, got %d", len(expectedEntries), len(allEntries))
	}
}

// TestIndexWithLargeSegmentIDs tests index with large segment IDs
func TestIndexWithLargeSegmentIDs(t *testing.T) {
	im := storage.NewIndexManager()

	// Add entries with large segment IDs
	im.AddEntry(1000, 1000, 100)
	im.AddEntry(2000, 2000, 200)
	im.AddEntry(3000, 3000, 300)

	offset, err := im.GetSegmentOffset(2000)
	if err != nil {
		t.Fatalf("GetSegmentOffset failed: %v", err)
	}

	if offset != 200 {
		t.Errorf("Expected offset 200, got %d", offset)
	}
}

// TestIndexWithDenseTimestamps tests index with closely spaced timestamps
func TestIndexWithDenseTimestamps(t *testing.T) {
	im := storage.NewIndexManager()

	// Add entries with closely spaced timestamps
	for i := 0; i < 1000; i++ {
		im.AddEntry(int32(i), int64(1000+i), int64(100+i))
	}

	if im.GetSegmentCount() != 1000 {
		t.Errorf("Expected 1000 segments, got %d", im.GetSegmentCount())
	}

	// Find range
	segments := im.FindSegmentsInTimeRange(1500, 1600)
	if len(segments) == 0 {
		t.Error("Expected segments in range, got 0")
	}
}

// TestIndexWithSparseTimestamps tests index with sparse timestamps
func TestIndexWithSparseTimestamps(t *testing.T) {
	im := storage.NewIndexManager()

	// Add entries with large gaps
	im.AddEntry(1, 1000, 100)
	im.AddEntry(2, 10000, 200)
	im.AddEntry(3, 100000, 300)
	im.AddEntry(4, 1000000, 400)

	// Find in gap
	segments := im.FindSegmentsInTimeRange(5000, 50000)
	if len(segments) == 0 {
		t.Error("Expected segments in range, got 0")
	}
}

// TestSparseIndexFindSegments tests SparseIndex FindSegmentsForTimeRange
func TestSparseIndexFindSegments(t *testing.T) {
	index := models.SparseTimestampIndex{
		Entries: []models.IndexEntry{
			{Timestamp: 1000, SegmentID: 1, Offset: 100},
			{Timestamp: 2000, SegmentID: 2, Offset: 200},
			{Timestamp: 3000, SegmentID: 3, Offset: 300},
			{Timestamp: 4000, SegmentID: 4, Offset: 400},
		},
		TotalSegments: 4,
	}

	testCases := []struct {
		name        string
		startTime   int64
		endTime     int64
		minSegments int
	}{
		{"full range", 0, 5000, 1},
		{"partial range", 2000, 3500, 1},
		{"single timestamp", 2000, 2000, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := index.FindSegmentsForTimeRange(tc.startTime, tc.endTime)
			if len(result) < tc.minSegments {
				t.Errorf("Expected at least %d segments, got %d", tc.minSegments, len(result))
			}
		})
	}
}

// TestSparseIndexGetSegmentOffset tests SparseIndex GetSegmentOffset
func TestSparseIndexGetSegmentOffset(t *testing.T) {
	index := models.SparseTimestampIndex{
		Entries: []models.IndexEntry{
			{Timestamp: 1000, SegmentID: 1, Offset: 100},
			{Timestamp: 2000, SegmentID: 2, Offset: 200},
		},
		TotalSegments: 2,
	}

	offset := index.GetSegmentOffset(1)
	if offset != 100 {
		t.Errorf("Expected offset 100, got %d", offset)
	}

	offset = index.GetSegmentOffset(999)
	if offset != -1 {
		t.Errorf("Expected offset -1 for non-existent segment, got %d", offset)
	}
}

// TestSparseIndexTimestampBounds tests SparseIndex timestamp bounds
func TestSparseIndexTimestampBounds(t *testing.T) {
	index := models.SparseTimestampIndex{
		Entries: []models.IndexEntry{
			{Timestamp: 1000, SegmentID: 1},
			{Timestamp: 5000, SegmentID: 5},
		},
		TotalSegments: 2,
	}

	minTime := index.GetMinTimestamp()
	if minTime != 1000 {
		t.Errorf("Expected min timestamp 1000, got %d", minTime)
	}

	maxTime := index.GetMaxTimestamp()
	if maxTime != 5000 {
		t.Errorf("Expected max timestamp 5000, got %d", maxTime)
	}
}
