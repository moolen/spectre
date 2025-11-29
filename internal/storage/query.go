package storage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// QueryExecutor executes queries against stored events
type QueryExecutor struct {
	logger       *logging.Logger
	storage      *Storage
	filterEngine *FilterEngine
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(storage *Storage) *QueryExecutor {
	return &QueryExecutor{
		logger:       logging.GetLogger("query"),
		storage:      storage,
		filterEngine: NewFilterEngine(),
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

	// Query in-memory buffered events (unflushed data) for low-latency queries
	qe.logger.Debug("Querying in-memory buffered events")
	inMemoryEvents, err := qe.storage.GetInMemoryEvents(query)
	if err != nil {
		qe.logger.Warn("Failed to query in-memory events: %v", err)
	} else {
		qe.logger.Debug("Found %d events in memory", len(inMemoryEvents))
		allEvents = append(allEvents, inMemoryEvents...)
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

// queryFile queries a single storage file (supports both block storage and legacy segment formats)
func (qe *QueryExecutor) queryFile(filePath string, query *models.QueryRequest) ([]models.Event, int32, int32, error) {
	// Try to open file using BlockReader (block storage format)
	reader, err := NewBlockReader(filePath)
	if err != nil {
		// If it's not a valid block storage file, try legacy segment format
		if isInvalidBlockFormatError(err) {
			return nil, 0, 0, fmt.Errorf("file %s is not block storage format", filePath)
		}
		// If file is incomplete (no footer), try to query it
		if isIncompleteFileError(err) {
			qe.logger.Debug("File %s is incomplete, attempting to query by scanning segments", filePath)
			return qe.queryIncompleteFile(filePath, query)
		}
		return nil, 0, 0, err
	}
	defer reader.Close()

	// Read complete file structure
	fileData, err := reader.ReadFile()
	if err != nil {
		reader.Close() // Close before trying legacy format
		// If it's not a valid block storage file, try legacy segment format
		if isInvalidBlockFormatError(err) {
			return nil, 0, 0, fmt.Errorf("file %s is not block storage format", filePath)
		}
		if isIncompleteFileError(err) {
			qe.logger.Debug("File %s is incomplete, attempting to query by scanning segments", filePath)
			return qe.queryIncompleteFile(filePath, query)
		}
		return nil, 0, 0, err
	}

	qe.logger.Debug("File %s: has %d blocks", filePath, len(fileData.IndexSection.BlockMetadata))

	var results []models.Event
	var segmentsScanned int32
	var segmentsSkipped int32

	// Query time range in nanoseconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	// Iterate through blocks
	for _, blockMeta := range fileData.IndexSection.BlockMetadata {
		// Check if block overlaps with query time range
		if blockMeta.TimestampMax < startTimeNs || blockMeta.TimestampMin > endTimeNs {
			qe.logger.Debug("File %s: skipping block %d (time range [%d, %d] outside query [%d, %d])",
				filePath, blockMeta.ID, blockMeta.TimestampMin, blockMeta.TimestampMax, startTimeNs, endTimeNs)
			segmentsSkipped++
			continue
		}

		// Check if block matches resource filters using metadata
		if !qe.blockMatchesFilters(blockMeta, query.Filters) {
			qe.logger.Debug("File %s: skipping block %d (metadata doesn't match filters)", filePath, blockMeta.ID)
			segmentsSkipped++
			continue
		}

		// Read and decompress block
		events, err := reader.ReadBlockEvents(blockMeta)
		if err != nil {
			qe.logger.Warn("Failed to read block %d from file %s: %v", blockMeta.ID, filePath, err)
			segmentsSkipped++
			continue
		}

		segmentsScanned++

		// Filter events by time range and resource filters
		var blockEvents []models.Event
		for _, event := range events {
			if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
				continue
			}

			if !qe.filterEngine.MatchesFilters(event, query.Filters) {
				continue
			}

			blockEvents = append(blockEvents, *event)
		}

		qe.logger.Debug("File %s: block %d loaded %d events (after filtering)", filePath, blockMeta.ID, len(blockEvents))
		results = append(results, blockEvents...)
	}

	return results, segmentsScanned, segmentsSkipped, nil
}

// isInvalidBlockFormatError checks if an error indicates the file is not in block storage format
func isInvalidBlockFormatError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "invalid file header magic bytes")
}

// QueryCount returns the number of events matching the query
func (qe *QueryExecutor) QueryCount(query *models.QueryRequest) (int64, error) {
	result, err := qe.Execute(query)
	if err != nil {
		return 0, err
	}
	return int64(result.Count), nil
}

// queryIncompleteFile queries an incomplete block storage file
// Block storage files require the index section to locate blocks, so truly incomplete files
// (still being written with no footer) cannot be queried - we skip them.
// This function attempts to read the file as a complete block storage file first.
func (qe *QueryExecutor) queryIncompleteFile(filePath string, query *models.QueryRequest) ([]models.Event, int32, int32, error) {
	// Try to read file using BlockReader - it may actually be complete
	reader, err := NewBlockReader(filePath)
	if err != nil {
		// File is truly incomplete or corrupted
		qe.logger.Debug("File %s cannot be read: %v", filePath, err)
		return []models.Event{}, 0, 0, nil
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		// File is incomplete - no valid footer/index
		qe.logger.Debug("File %s has no valid index: %v", filePath, err)
		return []models.Event{}, 0, 0, nil
	}

	// File is actually complete! Query it using block metadata
	var allEvents []models.Event
	var blocksScanned int32
	var blocksSkipped int32

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	for _, blockMeta := range fileData.IndexSection.BlockMetadata {
		// Check if block overlaps with query time range
		if blockMeta.TimestampMax < startTimeNs || blockMeta.TimestampMin > endTimeNs {
			blocksSkipped++
			continue
		}

		// Check if block matches resource filters using metadata
		if !qe.blockMatchesFilters(blockMeta, query.Filters) {
			blocksSkipped++
			continue
		}

		// Read and decompress block
		events, err := reader.ReadBlockEvents(blockMeta)
		if err != nil {
			qe.logger.Warn("Failed to read block %d from file %s: %v", blockMeta.ID, filePath, err)
			blocksSkipped++
			continue
		}

		// Filter events by time range and resource filters
		for _, event := range events {
			if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
				continue
			}

			if !qe.filterEngine.MatchesFilters(event, query.Filters) {
				continue
			}

			allEvents = append(allEvents, *event)
		}

		blocksScanned++
	}

	qe.logger.Debug("File %s (incomplete): scanned %d blocks, skipped %d, found %d events",
		filePath, blocksScanned, blocksSkipped, len(allEvents))

	return allEvents, blocksScanned, blocksSkipped, nil
}

// blockMatchesFilters checks if a block might contain matching events based on its metadata
func (qe *QueryExecutor) blockMatchesFilters(blockMeta *BlockMetadata, filters models.QueryFilters) bool {
	// If no filters, block matches
	if filters.Kind == "" && filters.Namespace == "" && filters.Group == "" {
		return true
	}

	// Check kind filter
	if filters.Kind != "" {
		found := false
		for _, kind := range blockMeta.KindSet {
			if kind == filters.Kind {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check namespace filter
	if filters.Namespace != "" {
		found := false
		for _, ns := range blockMeta.NamespaceSet {
			if ns == filters.Namespace {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check group filter
	if filters.Group != "" {
		found := false
		for _, group := range blockMeta.GroupSet {
			if group == filters.Group {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
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
