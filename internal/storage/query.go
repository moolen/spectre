package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// QueryExecutor executes queries against stored events
type QueryExecutor struct {
	logger        *logging.Logger
	storage       *Storage
	filterEngine  *FilterEngine
	metadataIndex *SegmentMetadataIndex
	indexManager  *IndexManager
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(storage *Storage) *QueryExecutor {
	return &QueryExecutor{
		logger:        logging.GetLogger("query"),
		storage:       storage,
		filterEngine:  NewFilterEngine(),
		metadataIndex: NewSegmentMetadataIndex(),
		indexManager:  NewIndexManager(),
	}
}

// Execute executes a query against stored events
func (qe *QueryExecutor) Execute(query *models.QueryRequest) (*models.QueryResult, error) {
	start := time.Now()

	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	qe.logger.DebugWithFields("Executing query",
		logging.Field("start_timestamp", query.StartTimestamp),
		logging.Field("end_timestamp", query.EndTimestamp),
		logging.Field("filters", fmt.Sprintf("%v", query.Filters)))

	// Find all storage files that overlap with the time range
	files, err := qe.storage.getStorageFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage files: %w", err)
	}

	qe.logger.Debug("Found %d storage files to search", len(files))
	for _, f := range files {
		qe.logger.Debug("  - %s", f)
	}

	var allEvents []models.Event
	filesSearched := 0
	totalSegmentsScanned := int32(0)
	totalSegmentsSkipped := int32(0)

	// Query each file
	for _, filePath := range files {
		qe.logger.Debug("Querying file: %s", filePath)
		events, segmentsScanned, segmentsSkipped, err := qe.queryFile(filePath, query)
		if err != nil {
			// Skip incomplete files (still being written) - this is expected for the current hour's file
			if isIncompleteFileError(err) {
				qe.logger.Debug("Skipping incomplete file %s (still being written)", filePath)
			} else {
				qe.logger.Warn("Failed to query file %s: %v", filePath, err)
			}
			continue
		}

		allEvents = append(allEvents, events...)
		filesSearched++
		totalSegmentsScanned += segmentsScanned
		totalSegmentsSkipped += segmentsSkipped
	}

	// Filter events by time range (in case of boundary issues)
	// Events use nanoseconds, but FilterByTimeRange expects nanoseconds, so convert query to nanoseconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9
	allEvents = qe.filterEngine.FilterByTimeRange(allEvents, startTimeNs, endTimeNs)

	// Sort events by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	// Create result
	executionTime := time.Since(start)
	result := &models.QueryResult{
		Events:          allEvents,
		Count:           int32(len(allEvents)),
		ExecutionTimeMs: int32(executionTime.Milliseconds()),
		SegmentsScanned: totalSegmentsScanned,
		SegmentsSkipped: totalSegmentsSkipped,
		FilesSearched:   int32(filesSearched),
	}

	qe.logger.InfoWithFields("Query complete",
		logging.Field("events_found", result.Count),
		logging.Field("execution_time_ms", result.ExecutionTimeMs),
		logging.Field("files_searched", filesSearched),
		logging.Field("total_files", len(files)),
		logging.Field("segments_scanned", totalSegmentsScanned),
		logging.Field("segments_skipped", totalSegmentsSkipped))

	if result.Count == 0 && len(files) > 0 {
		qe.logger.Info("No events found. Check debug logs for details on why segments/events were filtered out.")
	}

	return result, nil
}

// queryFile queries a single storage file
func (qe *QueryExecutor) queryFile(filePath string, query *models.QueryRequest) ([]models.Event, int32, int32, error) {
	// Read file metadata and index
	_, index, err := qe.readFileMetadata(filePath)
	if err != nil {
		return nil, 0, 0, err
	}

	qe.logger.Debug("File %s: index has %d entries, %d total segments", filePath, len(index.Entries), index.TotalSegments)
	if len(index.Entries) > 0 {
		firstEntry := index.Entries[0]
		qe.logger.Debug("File %s: first index entry: segmentID=%d, timestamp=%d ns (%.2f s)",
			filePath, firstEntry.SegmentID, firstEntry.Timestamp, float64(firstEntry.Timestamp)/1e9)
	}

	var results []models.Event
	var segmentsScanned int32
	var segmentsSkipped int32

	// Find candidate segments based on time range
	// Index entries use nanoseconds (from event timestamps), query is in seconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9
	qe.logger.Debug("File %s: searching for segments in time range [%d ns (%.2f s), %d ns (%.2f s)]",
		filePath, startTimeNs, float64(startTimeNs)/1e9, endTimeNs, float64(endTimeNs)/1e9)

	// Log index entries for debugging
	if len(index.Entries) > 0 {
		qe.logger.Debug("File %s: index entries:", filePath)
		for i, entry := range index.Entries {
			qe.logger.Debug("  [%d] segmentID=%d, timestamp=%d ns (%.2f s), offset=%d",
				i, entry.SegmentID, entry.Timestamp, float64(entry.Timestamp)/1e9, entry.Offset)
		}
	}

	candidateSegments := index.FindSegmentsForTimeRange(startTimeNs, endTimeNs)
	qe.logger.Debug("File %s: found %d candidate segments", filePath, len(candidateSegments))
	for _, segID := range candidateSegments {
		qe.logger.Debug("File %s: candidate segment ID: %d", filePath, segID)
	}

	// Open the file for reading segments
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create compressor for decompression
	compressor := NewCompressor()

	// Filter segments based on resource filters and load events
	for _, segmentID := range candidateSegments {
		// Get segment offset from index
		offset := index.GetSegmentOffset(segmentID)
		if offset < 0 {
			qe.logger.Warn("Segment %d not found in index for file %s", segmentID, filePath)
			segmentsSkipped++
			continue
		}

		// Check segment metadata to skip non-matching segments (if available)
		// Note: metadataIndex is not currently populated from file, so we skip this check
		// TODO: Populate metadataIndex from file metadata when reading
		if qe.metadataIndex != nil {
			if _, ok := qe.metadataIndex.GetSegmentMetadata(segmentID); ok {
				qe.logger.Debug("File %s: segment %d has metadata, checking if can skip", filePath, segmentID)
				if qe.metadataIndex.CanSegmentBeSkipped(segmentID, query.Filters) {
					qe.logger.Debug("File %s: skipping segment %d based on metadata filters", filePath, segmentID)
					segmentsSkipped++
					continue
				}
			} else {
				qe.logger.Debug("File %s: segment %d has no metadata in index, will load anyway", filePath, segmentID)
			}
		}

		segmentsScanned++

		// Load and filter events from segment
		qe.logger.Debug("File %s: loading segment %d from offset %d", filePath, segmentID, offset)
		segmentEvents, err := qe.loadSegmentEvents(file, offset, compressor, query)
		if err != nil {
			qe.logger.Warn("Failed to load segment %d from file %s: %v", segmentID, filePath, err)
			continue
		}

		qe.logger.Debug("File %s: segment %d loaded %d events", filePath, segmentID, len(segmentEvents))
		results = append(results, segmentEvents...)
	}

	return results, segmentsScanned, segmentsSkipped, nil
}

// loadSegmentEvents loads events from a segment at the given offset
func (qe *QueryExecutor) loadSegmentEvents(file *os.File, offset int64, compressor *Compressor, query *models.QueryRequest) ([]models.Event, error) {
	// Seek to segment offset
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to segment: %w", err)
	}

	// Read segment header
	// Format: ID(4) + startTimestamp(8) + endTimestamp(8) + eventCount(4) + uncompressedSize(8) + compressedSize(8) = 40 bytes
	var segmentID int32
	var startTimestamp, endTimestamp int64
	var eventCount int32
	var uncompressedSize, compressedSize int64

	if err := binary.Read(file, binary.LittleEndian, &segmentID); err != nil {
		return nil, fmt.Errorf("failed to read segment ID: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &startTimestamp); err != nil {
		return nil, fmt.Errorf("failed to read start timestamp: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &endTimestamp); err != nil {
		return nil, fmt.Errorf("failed to read end timestamp: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &eventCount); err != nil {
		return nil, fmt.Errorf("failed to read event count: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &uncompressedSize); err != nil {
		return nil, fmt.Errorf("failed to read uncompressed size: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &compressedSize); err != nil {
		return nil, fmt.Errorf("failed to read compressed size: %w", err)
	}

	// Quick check: if segment is completely outside time range, skip it
	// Segment timestamps are in nanoseconds (from event timestamps), query is in seconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9
	if endTimestamp < startTimeNs || startTimestamp > endTimeNs {
		qe.logger.Debug("Segment %d: outside time range [%d, %d] ns, segment=[%d, %d] ns", segmentID, startTimeNs, endTimeNs, startTimestamp, endTimestamp)
		return []models.Event{}, nil
	}

	// Read compressed data
	compressedData := make([]byte, compressedSize)
	if _, err := file.Read(compressedData); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Decompress the data
	decompressed, err := compressor.Decompress(compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress segment data: %w", err)
	}

	// Parse events from newline-delimited JSON
	var allEvents []models.Event
	lines := bytes.Split(decompressed, []byte("\n"))
	qe.logger.Debug("Segment %d: decompressed %d bytes, split into %d lines", segmentID, len(decompressed), len(lines))

	var eventsParsed, eventsFilteredTime, eventsFilteredResource int

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			qe.logger.Warn("Failed to unmarshal event: %v", err)
			continue
		}
		eventsParsed++

		// Filter by time range
		if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
			eventsFilteredTime++
			continue
		}

		// Filter by resource filters
		if !qe.filterEngine.MatchesFilters(&event, query.Filters) {
			eventsFilteredResource++
			continue
		}

		allEvents = append(allEvents, event)
	}

	qe.logger.Debug("Segment events: parsed=%d, filtered_time=%d, filtered_resource=%d, final=%d",
		eventsParsed, eventsFilteredTime, eventsFilteredResource, len(allEvents))

	return allEvents, nil
}

// readFileMetadata reads metadata and index from a file
func (qe *QueryExecutor) readFileMetadata(filePath string) (models.FileMetadata, models.SparseTimestampIndex, error) {
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	// Read the footer to find where metadata starts
	// Footer is "RPK_EOF" (7 bytes)
	footerSize := int64(7)

	// Check if file is large enough to have a footer
	// Minimum file size: header + at least footer marker
	minFileSize := footerSize
	if fileSize < minFileSize {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("file too small or incomplete (size: %d, minimum: %d)", fileSize, minFileSize)
	}

	// Seek to read footer
	footerOffset := fileSize - footerSize
	if footerOffset < 0 {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("file too small to contain footer (size: %d)", fileSize)
	}

	if _, err := file.Seek(footerOffset, 0); err != nil {
		// File might be incomplete (still being written) - return a specific error
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("file incomplete or still being written: %w", err)
	}

	footer := make([]byte, footerSize)
	if _, err := file.Read(footer); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read footer: %w", err)
	}

	if string(footer) != "RPK_EOF" {
		// Footer doesn't exist yet - file is incomplete
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("file incomplete: footer not found (file may still be being written)")
	}

	// File structure: [data] [metadata_len(4)] [metadata] [index_len(4)] [index] [footer(7)]
	// To read backwards, we need to find where the index JSON starts
	// The index JSON ends with } right before RPK_EOF, so we search backwards for {"entries
	// Read a reasonable chunk before the footer to find the index JSON start
	searchWindow := int64(1000) // Read up to 1KB before footer to find index
	if fileSize < searchWindow+footerSize {
		searchWindow = fileSize - footerSize
	}

	readStart := fileSize - footerSize - searchWindow
	if readStart < 0 {
		readStart = 0
	}

	file.Seek(readStart, 0)
	searchData := make([]byte, searchWindow)
	if _, err := file.Read(searchData); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read search window: %w", err)
	}

	// Find the index JSON start by looking for {"entries
	indexStartMarker := []byte(`{"entries`)
	indexStartInWindow := bytes.LastIndex(searchData, indexStartMarker)
	if indexStartInWindow < 0 {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("index JSON start marker not found")
	}

	// Calculate absolute position of index JSON start
	indexJSONStart := readStart + int64(indexStartInWindow)

	// The index length is 4 bytes before the index JSON
	indexLenPos := indexJSONStart - 4
	if indexLenPos < 0 {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("index length position is negative")
	}

	// Read index length
	file.Seek(indexLenPos, 0)
	var indexLen int32
	if err := binary.Read(file, binary.LittleEndian, &indexLen); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read index length: %w", err)
	}

	if indexLen <= 0 || indexLen > 10*1024*1024 { // Sanity check: max 10MB index
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("invalid index length: %d", indexLen)
	}

	// Verify the index JSON length matches (index ends right before footer)
	expectedIndexLen := fileSize - footerSize - indexJSONStart
	if int64(indexLen) != expectedIndexLen {
		qe.logger.Debug("Index length mismatch: stored=%d, expected=%d, using stored value", indexLen, expectedIndexLen)
	}

	// Read index data
	file.Seek(indexJSONStart, 0)
	indexData := make([]byte, indexLen)
	if _, err := file.Read(indexData); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read index data: %w", err)
	}

	// Parse index JSON
	var index models.SparseTimestampIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// Read metadata length and data
	// Metadata ends right before the index length
	// Structure: [metadata_len(4)] [metadata] [index_len(4)] [index] [footer]
	// We need to find where metadata JSON starts by searching backwards from index length position
	metadataSearchStart := indexLenPos - 4
	metadataSearchWindow := int64(2000) // Search up to 2KB before index length
	if metadataSearchStart < metadataSearchWindow {
		metadataSearchWindow = metadataSearchStart
	}
	
	metadataReadStart := metadataSearchStart - metadataSearchWindow
	if metadataReadStart < 0 {
		metadataReadStart = 0
	}
	
	file.Seek(metadataReadStart, 0)
	metadataSearchData := make([]byte, metadataSearchWindow)
	if _, err := file.Read(metadataSearchData); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read metadata search window: %w", err)
	}
	
	// Find where metadata JSON ends (right before index length)
	// The metadata JSON should end with } right before the index length position
	// Calculate relative position of index length in our search window
	indexLenPosInWindow := metadataSearchStart - metadataReadStart
	
	// Find the last } before the index length position
	metadataEndInWindow := bytes.LastIndex(metadataSearchData[:indexLenPosInWindow], []byte("}"))
	if metadataEndInWindow < 0 {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("metadata JSON end marker not found")
	}
	
	// Find where metadata JSON starts (look for { before the end)
	metadataStartInWindow := bytes.LastIndex(metadataSearchData[:metadataEndInWindow+1], []byte("{"))
	if metadataStartInWindow < 0 {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("metadata JSON start marker not found")
	}
	
	// Calculate absolute positions
	metadataJSONStart := metadataReadStart + int64(metadataStartInWindow)
	metadataJSONEnd := metadataReadStart + int64(metadataEndInWindow) + 1 // +1 to include the }
	metadataLenActual := metadataJSONEnd - metadataJSONStart
	
	// Metadata length is 4 bytes before metadata JSON start
	metadataLenPos := metadataJSONStart - 4
	if metadataLenPos < 0 {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("metadata length position is negative")
	}
	
	file.Seek(metadataLenPos, 0)
	var metadataLen int32
	if err := binary.Read(file, binary.LittleEndian, &metadataLen); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read metadata length: %w", err)
	}
	
	if metadataLen <= 0 || metadataLen > 10*1024*1024 { // Sanity check: max 10MB metadata
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("invalid metadata length: %d", metadataLen)
	}
	
	// Verify the length matches what we found
	if int64(metadataLen) != metadataLenActual {
		qe.logger.Debug("Metadata length mismatch: stored=%d, actual=%d, using stored value", metadataLen, metadataLenActual)
	}
	
	// Read metadata data
	file.Seek(metadataJSONStart, 0)
	metadataData := make([]byte, metadataLen)
	if _, err := file.Read(metadataData); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read metadata data: %w", err)
	}
	
	// Verify metadata ends where index length starts
	expectedMetadataEnd := indexLenPos
	actualMetadataEnd := metadataJSONStart + int64(metadataLen)
	if actualMetadataEnd != expectedMetadataEnd {
		qe.logger.Debug("Metadata end mismatch: expected=%d, actual=%d", expectedMetadataEnd, actualMetadataEnd)
	}

	// Parse metadata JSON
	var metadata models.FileMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return metadata, index, nil
}

// QueryCount returns the number of events matching the query
func (qe *QueryExecutor) QueryCount(query *models.QueryRequest) (int64, error) {
	result, err := qe.Execute(query)
	if err != nil {
		return 0, err
	}
	return int64(result.Count), nil
}

// QueryForSegments queries and returns segments that match the query
func (qe *QueryExecutor) QueryForSegments(query *models.QueryRequest) ([]models.StorageSegment, error) {
	files, err := qe.storage.getStorageFiles()
	if err != nil {
		return nil, err
	}

	var segments []models.StorageSegment

	for _, filePath := range files {
		fileSegments, err := qe.getFileSegments(filePath, query)
		if err != nil {
			qe.logger.Warn("Failed to get segments from file %s: %v", filePath, err)
			continue
		}
		segments = append(segments, fileSegments...)
	}

	return segments, nil
}

// getFileSegments gets segments from a file that match the query
func (qe *QueryExecutor) getFileSegments(filePath string, query *models.QueryRequest) ([]models.StorageSegment, error) {
	// TODO: Implement segment retrieval
	return []models.StorageSegment{}, nil
}

// isIncompleteFileError checks if an error indicates the file is incomplete (still being written)
func isIncompleteFileError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "file too small") ||
		strings.Contains(errStr, "incomplete") ||
		strings.Contains(errStr, "footer not found") ||
		(strings.Contains(errStr, "invalid argument") && strings.Contains(errStr, "seek"))
}
