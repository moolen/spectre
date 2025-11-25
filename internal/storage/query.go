package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// QueryExecutor executes queries against stored events
type QueryExecutor struct {
	logger            *logging.Logger
	storage           *Storage
	filterEngine      *FilterEngine
	metadataIndex     *SegmentMetadataIndex
	indexManager      *IndexManager
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

	qe.logger.Debug("Executing query: start=%d, end=%d, filters=%v",
		query.StartTimestamp, query.EndTimestamp, query.Filters)

	// Find all storage files that overlap with the time range
	files, err := qe.storage.getStorageFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage files: %w", err)
	}

	var allEvents []models.Event
	filesSearched := 0
	totalSegmentsScanned := int32(0)
	totalSegmentsSkipped := int32(0)

	// Query each file
	for _, filePath := range files {
		events, segmentsScanned, segmentsSkipped, err := qe.queryFile(filePath, query)
		if err != nil {
			qe.logger.Warn("Failed to query file %s: %v", filePath, err)
			continue
		}

		allEvents = append(allEvents, events...)
		filesSearched++
		totalSegmentsScanned += segmentsScanned
		totalSegmentsSkipped += segmentsSkipped
	}

	// Filter events by time range (in case of boundary issues)
	allEvents = qe.filterEngine.FilterByTimeRange(allEvents, query.StartTimestamp, query.EndTimestamp)

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

	qe.logger.Info("Query complete: events=%d, executionTime=%dms, segmentsScanned=%d, segmentsSkipped=%d",
		result.Count, result.ExecutionTimeMs, totalSegmentsScanned, totalSegmentsSkipped)

	return result, nil
}

// queryFile queries a single storage file
func (qe *QueryExecutor) queryFile(filePath string, query *models.QueryRequest) ([]models.Event, int32, int32, error) {
	// Read file metadata and index
	_, index, err := qe.readFileMetadata(filePath)
	if err != nil {
		return nil, 0, 0, err
	}

	var results []models.Event
	var segmentsScanned int32
	var segmentsSkipped int32

	// Find candidate segments based on time range
	candidateSegments := index.FindSegmentsForTimeRange(query.StartTimestamp, query.EndTimestamp)

	// Filter segments based on resource filters
	for _, segmentID := range candidateSegments {
		// Check segment metadata to skip non-matching segments
		if _, ok := qe.metadataIndex.GetSegmentMetadata(segmentID); ok {
			if qe.metadataIndex.CanSegmentBeSkipped(segmentID, query.Filters) {
				segmentsSkipped++
				continue
			}
			segmentsScanned++

			// Load and filter events from segment
			offset, _ := qe.indexManager.GetSegmentOffset(segmentID)
			_ = offset // Will be used for actual segment loading in production

			// TODO: Load segment from file and filter events
		}
	}

	return results, segmentsScanned, segmentsSkipped, nil
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
	indexLengthSize := int64(4)
	metadataLengthSize := int64(4)

	// Seek to read footer
	if _, err := file.Seek(fileSize-footerSize, 0); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to seek footer: %w", err)
	}

	footer := make([]byte, footerSize)
	if _, err := file.Read(footer); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read footer: %w", err)
	}

	if string(footer) != "RPK_EOF" {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("invalid file format: footer mismatch")
	}

	// Seek back to read index length
	if _, err := file.Seek(fileSize-footerSize-indexLengthSize, 0); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to seek index length: %w", err)
	}

	var indexLen int32
	if err := binary.Read(file, binary.LittleEndian, &indexLen); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read index length: %w", err)
	}

	// Read index data
	indexData := make([]byte, indexLen)
	if _, err := file.Read(indexData); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read index data: %w", err)
	}

	// Parse index JSON
	var index models.SparseTimestampIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// Seek back to read metadata length
	if _, err := file.Seek(fileSize-footerSize-indexLengthSize-int64(indexLen)-metadataLengthSize, 0); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to seek metadata length: %w", err)
	}

	var metadataLen int32
	if err := binary.Read(file, binary.LittleEndian, &metadataLen); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read metadata length: %w", err)
	}

	// Read metadata data
	metadataData := make([]byte, metadataLen)
	if _, err := file.Read(metadataData); err != nil {
		return models.FileMetadata{}, models.SparseTimestampIndex{}, fmt.Errorf("failed to read metadata data: %w", err)
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
