package storage

import (
	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// SegmentMetadataIndex manages metadata for multiple segments to enable efficient filtering
type SegmentMetadataIndex struct {
	logger    *logging.Logger
	metadatas map[int32]models.SegmentMetadata
}

// NewSegmentMetadataIndex creates a new segment metadata index
func NewSegmentMetadataIndex() *SegmentMetadataIndex {
	return &SegmentMetadataIndex{
		logger:    logging.GetLogger("segment_metadata"),
		metadatas: make(map[int32]models.SegmentMetadata),
	}
}

// AddMetadata adds metadata for a segment
func (smi *SegmentMetadataIndex) AddMetadata(segmentID int32, metadata models.SegmentMetadata) {
	smi.metadatas[segmentID] = metadata
	smi.logger.Debug("Added metadata for segment %d", segmentID)
}

// CanSegmentBeSkipped checks if a segment can be skipped based on filters
func (smi *SegmentMetadataIndex) CanSegmentBeSkipped(segmentID int32, filters models.QueryFilters) bool {
	if metadata, ok := smi.metadatas[segmentID]; ok {
		// If filters are empty, segment cannot be skipped
		if filters.IsEmpty() {
			return false
		}

		// Check if segment matches filters
		return !metadata.MatchesFilters(filters)
	}

	// If metadata not found, cannot skip
	return false
}

// FilterSegments returns segments that match the given filters
func (smi *SegmentMetadataIndex) FilterSegments(segmentIDs []int32, filters models.QueryFilters) []int32 {
	var result []int32

	for _, segmentID := range segmentIDs {
		// If no metadata for segment, include it to be safe
		if metadata, ok := smi.metadatas[segmentID]; !ok {
			result = append(result, segmentID)
		} else {
			// Include if it matches filters
			if metadata.MatchesFilters(filters) {
				result = append(result, segmentID)
			}
		}
	}

	return result
}

// GetSegmentMetadata returns metadata for a specific segment
func (smi *SegmentMetadataIndex) GetSegmentMetadata(segmentID int32) (models.SegmentMetadata, bool) {
	metadata, ok := smi.metadatas[segmentID]
	return metadata, ok
}

// GetAllMetadatas returns all segment metadatas
func (smi *SegmentMetadataIndex) GetAllMetadatas() map[int32]models.SegmentMetadata {
	return smi.metadatas
}

// ContainsNamespace checks if any segment contains the given namespace
func (smi *SegmentMetadataIndex) ContainsNamespace(namespace string) bool {
	for _, metadata := range smi.metadatas {
		if metadata.ContainsNamespace(namespace) {
			return true
		}
	}
	return false
}

// ContainsKind checks if any segment contains the given kind
func (smi *SegmentMetadataIndex) ContainsKind(kind string) bool {
	for _, metadata := range smi.metadatas {
		if metadata.ContainsKind(kind) {
			return true
		}
	}
	return false
}

// GetNamespaces returns all unique namespaces across all segments
func (smi *SegmentMetadataIndex) GetNamespaces() []string {
	namespaceSet := make(map[string]bool)

	for _, metadata := range smi.metadatas {
		for ns := range metadata.NamespaceSet {
			namespaceSet[ns] = true
		}
	}

	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	return namespaces
}

// GetKinds returns all unique kinds across all segments
func (smi *SegmentMetadataIndex) GetKinds() []string {
	kindSet := make(map[string]bool)

	for _, metadata := range smi.metadatas {
		for kind := range metadata.KindSet {
			kindSet[kind] = true
		}
	}

	var kinds []string
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}

	return kinds
}

// GetSegmentCount returns the number of segments with metadata
func (smi *SegmentMetadataIndex) GetSegmentCount() int {
	return len(smi.metadatas)
}

// Clear removes all metadata
func (smi *SegmentMetadataIndex) Clear() {
	smi.metadatas = make(map[int32]models.SegmentMetadata)
	smi.logger.Debug("Segment metadata index cleared")
}
