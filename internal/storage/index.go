package storage

import (
	"fmt"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// IndexManager manages sparse timestamp indexes for segments
type IndexManager struct {
	logger  *logging.Logger
	entries map[int32]*models.IndexEntry
}

// NewIndexManager creates a new index manager
func NewIndexManager() *IndexManager {
	return &IndexManager{
		logger:  logging.GetLogger("index"),
		entries: make(map[int32]*models.IndexEntry),
	}
}

// AddEntry adds an index entry for a segment
func (im *IndexManager) AddEntry(segmentID int32, timestamp, offset int64) {
	entry := &models.IndexEntry{
		Timestamp: timestamp,
		SegmentID: segmentID,
		Offset:    offset,
	}
	im.entries[segmentID] = entry
	im.logger.Debug("Added index entry for segment %d at offset %d", segmentID, offset)
}

// FindSegmentsInTimeRange finds segments that might contain events in the given time range
func (im *IndexManager) FindSegmentsInTimeRange(startTime, endTime int64) []int32 {
	var segmentIDs []int32

	// Iterate through entries and find those in the time range
	for _, entry := range im.entries {
		// Check if this entry's timestamp is within or near the range
		// Since this is a sparse index, we need to be conservative
		if entry.Timestamp >= startTime && entry.Timestamp <= endTime {
			segmentIDs = append(segmentIDs, entry.SegmentID)
		} else if entry.Timestamp < startTime {
			// Include segments before the range as they might contain events at the start
			segmentIDs = append(segmentIDs, entry.SegmentID)
		}
	}

	return segmentIDs
}

// GetSegmentOffset returns the byte offset for a segment
func (im *IndexManager) GetSegmentOffset(segmentID int32) (int64, error) {
	if entry, ok := im.entries[segmentID]; ok {
		return entry.Offset, nil
	}
	return -1, fmt.Errorf("segment %d not found in index", segmentID)
}

// GetSegmentCount returns the number of indexed segments
func (im *IndexManager) GetSegmentCount() int {
	return len(im.entries)
}

// BuildSparseIndex builds a sparse index from the entries
func (im *IndexManager) BuildSparseIndex() models.SparseTimestampIndex {
	// Sort entries by timestamp (simplified - in production, would use proper sorting)
	entries := make([]models.IndexEntry, 0, len(im.entries))
	for _, entry := range im.entries {
		entries = append(entries, *entry)
	}

	return models.SparseTimestampIndex{
		Entries:       entries,
		TotalSegments: int32(len(im.entries)), //nolint:gosec // safe conversion: entry count is reasonable
	}
}

// GetEntriesInRange returns index entries within a timestamp range
func (im *IndexManager) GetEntriesInRange(startTime, endTime int64) []models.IndexEntry {
	var result []models.IndexEntry

	for _, entry := range im.entries {
		if entry.Timestamp >= startTime && entry.Timestamp <= endTime {
			result = append(result, *entry)
		}
	}

	return result
}

// Clear removes all entries from the index
func (im *IndexManager) Clear() {
	im.entries = make(map[int32]*models.IndexEntry)
	im.logger.Debug("Index cleared")
}

// GetAllEntries returns all index entries
func (im *IndexManager) GetAllEntries() []models.IndexEntry {
	entries := make([]models.IndexEntry, 0, len(im.entries))
	for _, entry := range im.entries {
		entries = append(entries, *entry)
	}
	return entries
}
