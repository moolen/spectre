package models

// StorageSegment is an atomic unit of compressed event data within a storage file
type StorageSegment struct {
	// ID is the sequential segment number within the file
	ID int32 `json:"id"`

	// StartTimestamp is the minimum event timestamp in the segment
	StartTimestamp int64 `json:"startTimestamp"`

	// EndTimestamp is the maximum event timestamp in the segment
	EndTimestamp int64 `json:"endTimestamp"`

	// EventCount is the number of events in the segment
	EventCount int32 `json:"eventCount"`

	// UncompressedSize is the total uncompressed bytes
	UncompressedSize int64 `json:"uncompressedSize"`

	// CompressedSize is the total compressed bytes
	CompressedSize int64 `json:"compressedSize"`

	// Offset is the byte offset in the file where the segment starts
	Offset int64 `json:"offset"`

	// Length is the byte length of the compressed segment
	Length int64 `json:"length"`

	// Metadata contains segment metadata for efficient filtering
	Metadata SegmentMetadata `json:"metadata"`
}

// Validate checks that the segment is well-formed
func (s *StorageSegment) Validate() error {
	// Check timestamp range
	if s.StartTimestamp > s.EndTimestamp {
		return NewValidationError("startTimestamp cannot be greater than endTimestamp")
	}

	// Check event count
	if s.EventCount < 1 {
		return NewValidationError("eventCount must be at least 1")
	}

	// Check size constraint
	if s.UncompressedSize < s.CompressedSize {
		return NewValidationError("compressedSize cannot be greater than uncompressedSize")
	}

	// Check offset and length are valid
	if s.Offset < 0 || s.Length < 0 {
		return NewValidationError("offset and length must be non-negative")
	}

	return nil
}

// GetCompressionRatio returns the compression ratio (0.0 to 1.0)
func (s *StorageSegment) GetCompressionRatio() float64 {
	if s.UncompressedSize == 0 {
		return 0.0
	}
	return float64(s.CompressedSize) / float64(s.UncompressedSize)
}

// GetCompressionSavings returns the bytes saved by compression
func (s *StorageSegment) GetCompressionSavings() int64 {
	return s.UncompressedSize - s.CompressedSize
}

// IsValid checks if the segment is valid
func (s *StorageSegment) IsValid() bool {
	return s.Validate() == nil
}

// OverlapsWith checks if this segment overlaps with a time range
func (s *StorageSegment) OverlapsWith(startTime, endTime int64) bool {
	return s.StartTimestamp <= endTime && s.EndTimestamp >= startTime
}
