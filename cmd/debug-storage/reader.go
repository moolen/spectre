package main

import (
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

// FileData contains all decoded file information
type FileData struct {
	Header            *storage.FileHeader
	Footer            *storage.FileFooter
	IndexSection      *storage.IndexSection
	Blocks            []*BlockData
	Events            []*models.Event
	Statistics        map[string]interface{}
	FinalResourceStates map[string]*storage.ResourceLastState
}

// BlockData contains decoded block information
type BlockData struct {
	Metadata *storage.BlockMetadata
	Events   []*models.Event
}

// ReadStorageFile reads and decodes a complete storage file
func ReadStorageFile(filePath string) (*FileData, error) {
	reader, err := storage.NewBlockReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create block reader: %w", err)
	}
	defer reader.Close()

	// Read file header
	header, err := reader.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	// Read file footer
	footer, err := reader.ReadFileFooter()
	if err != nil {
		return nil, fmt.Errorf("failed to read file footer: %w", err)
	}

	// Read index section
	indexSection, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read index section: %w", err)
	}

	// Read all blocks and events
	var blocks []*BlockData
	var allEvents []*models.Event

	for i := 0; i < len(indexSection.BlockMetadata); i++ {
		metadata := indexSection.BlockMetadata[i]

		// Read block events
		events, err := reader.ReadBlockEvents(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to read block %d: %w", i, err)
		}

		blocks = append(blocks, &BlockData{
			Metadata: metadata,
			Events:   events,
		})

		allEvents = append(allEvents, events...)
	}

	// Build statistics
	stats := buildStatistics(header, indexSection, blocks)

	return &FileData{
		Header:              header,
		Footer:              footer,
		IndexSection:        indexSection,
		Blocks:              blocks,
		Events:              allEvents,
		Statistics:          stats,
		FinalResourceStates: indexSection.FinalResourceStates,
	}, nil
}

func buildStatistics(header *storage.FileHeader, index *storage.IndexSection, blocks []*BlockData) map[string]interface{} {
	stats := make(map[string]interface{})

	// Basic file info
	stats["FormatVersion"] = header.FormatVersion
	stats["Compression"] = header.CompressionAlgorithm
	stats["Encoding"] = header.EncodingFormat
	stats["BlockSize"] = header.BlockSize
	stats["ChecksumEnabled"] = header.ChecksumEnabled
	stats["CreatedAt"] = time.Unix(0, header.CreatedAt).Format(time.RFC3339)

	// Count statistics
	stats["TotalBlocks"] = len(blocks)
	stats["TotalEvents"] = len(blocks)

	totalCompressed := int64(0)
	totalUncompressed := int64(0)
	uniqueKinds := make(map[string]bool)
	uniqueNamespaces := make(map[string]bool)
	uniqueGroups := make(map[string]bool)

	var minTimestamp int64 = 9223372036854775807 // max int64
	var maxTimestamp int64 = 0

	for _, blockData := range blocks {
		metadata := blockData.Metadata
		totalCompressed += metadata.CompressedLength
		totalUncompressed += metadata.UncompressedLength

		// Track timestamp range
		if metadata.TimestampMin > 0 && metadata.TimestampMin < minTimestamp {
			minTimestamp = metadata.TimestampMin
		}
		if metadata.TimestampMax > maxTimestamp {
			maxTimestamp = metadata.TimestampMax
		}

		// Collect unique values from the sets
		for _, kind := range metadata.KindSet {
			uniqueKinds[kind] = true
		}
		for _, ns := range metadata.NamespaceSet {
			uniqueNamespaces[ns] = true
		}
		for _, group := range metadata.GroupSet {
			uniqueGroups[group] = true
		}
	}

	// Reset min timestamp if it wasn't set
	if minTimestamp == 9223372036854775807 {
		minTimestamp = 0
	}

	stats["CompressedSize"] = totalCompressed
	stats["UncompressedSize"] = totalUncompressed
	if totalUncompressed > 0 {
		stats["CompressionRatio"] = float64(totalCompressed) / float64(totalUncompressed)
	}

	stats["UniqueKinds"] = len(uniqueKinds)
	stats["UniqueNamespaces"] = len(uniqueNamespaces)
	stats["UniqueGroups"] = len(uniqueGroups)

	// Time range
	stats["TimestampMin"] = minTimestamp
	stats["TimestampMax"] = maxTimestamp

	return stats
}
