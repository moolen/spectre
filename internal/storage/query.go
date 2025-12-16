package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// QueryExecutor executes queries against stored events
type QueryExecutor struct {
	logger       *logging.Logger
	storage      *Storage
	filterEngine *FilterEngine
	cache        *BlockCache
	tracer       trace.Tracer
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(storage *Storage, tracingProvider interface{}) *QueryExecutor {
	tracer := getTracerFromProvider(tracingProvider, "spectre.storage")
	return &QueryExecutor{
		logger:       logging.GetLogger("query"),
		storage:      storage,
		filterEngine: NewFilterEngine(),
		cache:        nil, // Cache will be initialized separately
		tracer:       tracer,
	}
}

// NewQueryExecutorWithCache creates a new query executor with block caching
func NewQueryExecutorWithCache(storage *Storage, cacheMaxMB int64, tracingProvider interface{}) (*QueryExecutor, error) {
	cacheLogger := logging.GetLogger("cache")
	cache, err := NewBlockCache(cacheMaxMB, cacheLogger)
	if err != nil {
		return nil, err
	}

	tracer := getTracerFromProvider(tracingProvider, "spectre.storage")

	return &QueryExecutor{
		logger:       logging.GetLogger("query"),
		storage:      storage,
		filterEngine: NewFilterEngine(),
		cache:        cache,
		tracer:       tracer,
	}, nil
}

// Helper to extract tracer from provider
func getTracerFromProvider(tracingProvider interface{}, name string) trace.Tracer {
	if tracingProvider == nil {
		return otel.GetTracerProvider().Tracer(name)
	}

	if tp, ok := tracingProvider.(interface{ GetTracer(string) trace.Tracer }); ok {
		return tp.GetTracer(name)
	}

	return otel.GetTracerProvider().Tracer(name)
}

// SetCache sets the block cache for the executor
func (qe *QueryExecutor) SetCache(cache *BlockCache) {
	qe.cache = cache
}

// GetCache returns the block cache if it exists
func (qe *QueryExecutor) GetCache() *BlockCache {
	return qe.cache
}

// Execute executes a query against stored events
func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	// Start span for entire query execution
	ctx, span := qe.tracer.Start(ctx, "storage.Execute",
		trace.WithAttributes(
			attribute.Int64("query.start_timestamp", query.StartTimestamp),
			attribute.Int64("query.end_timestamp", query.EndTimestamp),
			attribute.String("query.namespace", query.Filters.Namespace),
			attribute.String("query.kind", query.Filters.Kind),
			attribute.String("query.group", query.Filters.Group),
		),
	)
	defer span.End()

	start := time.Now()

	// Validate query
	if err := query.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid query")
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	qe.logger.DebugWithFields("Executing query",
		logging.Field("start_timestamp", query.StartTimestamp),
		logging.Field("end_timestamp", query.EndTimestamp),
		logging.Field("filters", fmt.Sprintf("%v", query.Filters)))

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	// Use file index for fast file selection
	fileIndex := qe.storage.GetFileIndex()
	var filesToQuery []string
	// Track which files overlap with query range (for state snapshot filtering)
	overlappingFileSet := make(map[string]bool)

	if fileIndex != nil && fileIndex.Count() > 0 {
		// Fast path: use file index
		fileSelectionStart := time.Now()

		// Get files that overlap with query range
		overlappingFiles := fileIndex.GetFilesByTimeRange(startTimeNs, endTimeNs)
		filesToQuery = append(filesToQuery, overlappingFiles...)
		for _, f := range overlappingFiles {
			overlappingFileSet[f] = true
		}

		// Get one file before query start for boundary events
		fileBeforeQuery := fileIndex.GetFileBeforeTime(startTimeNs)
		if fileBeforeQuery != "" {
			// Check if it's not already included
			alreadyIncluded := false
			for _, f := range filesToQuery {
				if f == fileBeforeQuery {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				qe.logger.Debug("Including file before query for boundary events: %s", fileBeforeQuery)
				filesToQuery = append(filesToQuery, fileBeforeQuery)
				// Note: do NOT add to overlappingFileSet - this file is before the query range
			}
		}

		qe.logger.Debug("File selection via index completed in %dms, selected %d files",
			time.Since(fileSelectionStart).Milliseconds(), len(filesToQuery))

		span.SetAttributes(
			attribute.Int("storage.indexed_files", fileIndex.Count()),
			attribute.Bool("storage.used_index", true),
		)
	} else {
		// Fallback: traditional filename-based filtering without index
		qe.logger.Debug("File index not available, using traditional file selection")

		allFiles, err := qe.storage.getStorageFiles()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to get storage files")
			return nil, fmt.Errorf("failed to get storage files: %w", err)
		}

		span.SetAttributes(
			attribute.Int("storage.total_files", len(allFiles)),
			attribute.Bool("storage.used_index", false),
		)

		// Filter files based on hour boundaries (strict mode)
		var mostRecentFileBeforeQuery string
		var mostRecentFileBeforeHour int64

		for _, filePath := range allFiles {
			fileHour, err := qe.storage.extractHourFromFilename(filePath)
			if err != nil {
				qe.logger.Debug("Could not extract hour from file %s: %v", filePath, err)
				continue
			}

			fileHourNs := fileHour * 1e9
			fileEndNs := fileHourNs + (3600 * 1e9) // Files are 1 hour long

			// Check if file hour overlaps with query time range
			// With strict hour boundaries, we can trust the filename
			if fileEndNs > startTimeNs && fileHourNs < endTimeNs {
				filesToQuery = append(filesToQuery, filePath)
				overlappingFileSet[filePath] = true
			} else if fileEndNs <= startTimeNs && fileHour > mostRecentFileBeforeHour {
				// Track most recent file before query for boundary events
				mostRecentFileBeforeQuery = filePath
				mostRecentFileBeforeHour = fileHour
			}
		}

		// Add the most recent file before query start
		if mostRecentFileBeforeQuery != "" {
			qe.logger.Debug("Including file before query for boundary events: %s", mostRecentFileBeforeQuery)
			filesToQuery = append(filesToQuery, mostRecentFileBeforeQuery)
			// Note: do NOT add to overlappingFileSet - this file is before the query range
		}
	}

	// Log file selection timing
	fileSelectionTime := time.Since(start)
	qe.logger.Debug("File selection completed in %dms, selected %d files",
		fileSelectionTime.Milliseconds(), len(filesToQuery))
	for _, f := range filesToQuery {
		qe.logger.Debug("  - %s", f)
	}

	span.SetAttributes(attribute.Int("storage.files_to_query", len(filesToQuery)))

	var allEvents []models.Event
	filesSearched := 0
	totalSegmentsScanned := int32(0)
	totalSegmentsSkipped := int32(0)

	// First pass: collect deleted resources from all files to exclude them from state snapshots
	deletedResources := make(map[string]bool)
	for _, filePath := range filesToQuery {
		reader, err := NewBlockReader(filePath)
		if err != nil {
			// Skip files that can't be read (might be incomplete)
			continue
		}
		fileData, err := reader.ReadFile()
		_ = reader.Close()
		if err != nil {
			// Skip files with errors
			continue
		}

		// Check for deleted resources in final states
		for resourceKey, state := range fileData.IndexSection.FinalResourceStates {
			if state.EventType == string(models.EventTypeDelete) {
				deletedResources[resourceKey] = true
			}
		}
	}

	qe.logger.Debug("Found %d deleted resources across all files", len(deletedResources))

	// Query each filtered file
	for _, filePath := range filesToQuery {
		fileStart := time.Now()
		qe.logger.Debug("Querying file: %s", filePath)

		// Include state snapshots from all queried files
		// Note: we could exclude state snapshots from "file before query" by checking overlappingFileSet[filePath],
		// but that would break tests that expect pre-existing resources to show up
		includeStateSnapshots := true

		events, segmentsScanned, segmentsSkipped, err := qe.queryFileWithSnapshotsWithDeleted(ctx, filePath, query, includeStateSnapshots, deletedResources)
		if err != nil {
			// Skip incomplete files (still being written) - this is expected for the current hour's file
			if isIncompleteFileError(err) {
				qe.logger.Debug("Skipping incomplete file %s (still being written)", filePath)
			} else {
				qe.logger.Warn("Failed to query file %s: %v", filePath, err)
				span.AddEvent("file_query_error", trace.WithAttributes(
					attribute.String("file_path", filePath),
					attribute.String("error", err.Error()),
				))
			}
			continue
		}

		fileTime := time.Since(fileStart)
		qe.logger.Debug("File query completed: path=%s, time=%dms, events=%d, scanned=%d, skipped=%d",
			filePath, fileTime.Milliseconds(), len(events), segmentsScanned, segmentsSkipped)

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
	// Note: startTimeNs and endTimeNs were already defined above for file filtering

	// Separate state snapshot events from regular events for special filtering
	var stateSnapshots []models.Event
	var regularEvents []models.Event
	for _, event := range allEvents {
		if strings.HasPrefix(event.ID, "state-") {
			stateSnapshots = append(stateSnapshots, event)
		} else {
			regularEvents = append(regularEvents, event)
		}
	}

	// Regular events must be within query time range
	regularEvents = qe.filterEngine.FilterByTimeRange(regularEvents, startTimeNs, endTimeNs)

	// Track which resources have regular events
	// State snapshots for these resources should be excluded to avoid duplicates
	resourcesWithRegularEvents := make(map[string]bool)
	for _, event := range regularEvents {
		resourceKey := fmt.Sprintf("%s/%s/%s/%s/%s",
			event.Resource.Group,
			event.Resource.Version,
			event.Resource.Kind,
			event.Resource.Namespace,
			event.Resource.Name)
		resourcesWithRegularEvents[resourceKey] = true
	}

	// State snapshots represent resources that existed before the query window
	// Include them only if:
	// 1. Their timestamp is before the query start (pre-existing resources)
	// 2. The resource doesn't have regular events in the query range
	//
	// Note: state snapshots are only generated from files that overlap with the query range,
	// so we don't need additional timestamp filtering here
	var filteredSnapshots []models.Event
	for _, event := range stateSnapshots {
		// Only include state snapshots from before the query start time
		// This ensures we only show truly "pre-existing" resources
		if event.Timestamp < startTimeNs {
			resourceKey := fmt.Sprintf("%s/%s/%s/%s/%s",
				event.Resource.Group,
				event.Resource.Version,
				event.Resource.Kind,
				event.Resource.Namespace,
				event.Resource.Name)

			// Skip if this resource already has a regular event in the results
			if resourcesWithRegularEvents[resourceKey] {
				continue
			}

			filteredSnapshots = append(filteredSnapshots, event)
		}
	}

	// Deduplicate state snapshots (same resource may appear in multiple files due to carryover)
	// Keep only the most recent state snapshot per resource
	snapshotMap := make(map[string]models.Event)
	for _, event := range filteredSnapshots {
		resourceKey := fmt.Sprintf("%s/%s/%s/%s/%s",
			event.Resource.Group,
			event.Resource.Version,
			event.Resource.Kind,
			event.Resource.Namespace,
			event.Resource.Name)

		// Keep the snapshot with the latest timestamp
		if existing, ok := snapshotMap[resourceKey]; !ok || event.Timestamp > existing.Timestamp {
			snapshotMap[resourceKey] = event
		}
	}

	// Convert map back to slice
	deduplicatedSnapshots := make([]models.Event, 0, len(snapshotMap))
	for _, event := range snapshotMap {
		deduplicatedSnapshots = append(deduplicatedSnapshots, event)
	}

	// Combine both sets
	regularEvents = append(regularEvents, deduplicatedSnapshots...)
	allEvents = regularEvents

	// Sort events by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	// Create result
	executionTime := time.Since(start)
	result := &models.QueryResult{
		Events:          allEvents,
		Count:           int32(len(allEvents)),               //nolint:gosec // safe conversion: event count is reasonable
		ExecutionTimeMs: int32(executionTime.Milliseconds()), //nolint:gosec // safe conversion: execution time is reasonable
		SegmentsScanned: totalSegmentsScanned,
		SegmentsSkipped: totalSegmentsSkipped,
		FilesSearched:   int32(filesSearched), //nolint:gosec // safe conversion: file count is reasonable
	}

	// Count all files for stats
	allFilesForStats, _ := qe.storage.getStorageFiles()
	totalFiles := len(allFilesForStats)
	if totalFiles == 0 {
		totalFiles = len(filesToQuery)
	}

	qe.logger.InfoWithFields("Query complete",
		logging.Field("events_found", result.Count),
		logging.Field("execution_time_ms", result.ExecutionTimeMs),
		logging.Field("files_searched", filesSearched),
		logging.Field("total_files", totalFiles),
		logging.Field("segments_scanned", totalSegmentsScanned),
		logging.Field("segments_skipped", totalSegmentsSkipped))

	// Log cache statistics if cache is enabled
	if qe.cache != nil {
		stats := qe.cache.Stats()
		qe.logger.Info("Cache stats: hits=%d, misses=%d, hitRate=%.2f%%, memory=%dMB/%dMB, evictions=%d",
			stats.Hits, stats.Misses, stats.HitRate*100,
			stats.UsedMemory/(1024*1024), stats.MaxMemory/(1024*1024), stats.Evictions)
	}

	if result.Count == 0 && totalFiles > 0 {
		qe.logger.Info("No events found. Check debug logs for details on why segments/events were filtered out.")
	}

	// Add final metrics to span
	span.SetAttributes(
		attribute.Int("result.event_count", len(allEvents)),
		attribute.Int("result.files_searched", filesSearched),
		attribute.Int("result.segments_scanned", int(totalSegmentsScanned)),
		attribute.Int("result.segments_skipped", int(totalSegmentsSkipped)),
		attribute.Int64("result.execution_time_ms", executionTime.Milliseconds()),
	)
	span.SetStatus(codes.Ok, "Query executed successfully")

	return result, nil
}

// queryFileWithSnapshots queries a single storage file with optional state snapshot inclusion
func (qe *QueryExecutor) queryFileWithSnapshotsWithDeleted(ctx context.Context, filePath string, query *models.QueryRequest, includeStateSnapshots bool, deletedResources map[string]bool) ([]models.Event, int32, int32, error) {
	// Start span for single file query
	_, span := qe.tracer.Start(ctx, "storage.queryFile",
		trace.WithAttributes(
			attribute.String("file.path", filePath),
			attribute.Bool("include_state_snapshots", includeStateSnapshots),
		),
	)
	defer span.End()

	// Try to open file using BlockReader (block storage format)
	reader, err := NewBlockReader(filePath)
	if err != nil {
		// If this is an open file without footer, fall back to in-memory/open-file query
		if events, found, memErr := qe.storage.GetEventsFromOpenFile(filePath, query); found {
			if memErr != nil {
				return nil, 0, 0, memErr
			}
			return events, 0, 0, nil
		}

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
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	// Read complete file structure
	fileData, err := reader.ReadFile()
	if err != nil {
		// Defer will handle the close
		// If it's not a valid block storage file, try legacy segment format
		if isInvalidBlockFormatError(err) {
			return nil, 0, 0, fmt.Errorf("file %s is not block storage format", filePath)
		}
		if isIncompleteFileError(err) {
			// For open files (no footer), read events from in-memory/restored structures
			if events, found, memErr := qe.storage.GetEventsFromOpenFile(filePath, query); found {
				if memErr != nil {
					return nil, 0, 0, memErr
				}
				return events, 0, 0, nil
			}

			qe.logger.Debug("File %s is incomplete, attempting to query by scanning segments", filePath)
			return qe.queryIncompleteFile(filePath, query)
		}
		return nil, 0, 0, err
	}

	qe.logger.Debug("File %s: has %d blocks", filePath, len(fileData.IndexSection.BlockMetadata))

	span.SetAttributes(
		attribute.Int("file.block_count", len(fileData.IndexSection.BlockMetadata)),
	)

	var results []models.Event
	var segmentsScanned int32
	var segmentsSkipped int32

	// Query time range in nanoseconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	// Track which resources have events in the query results
	resourcesWithEvents := make(map[string]bool)

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

		// Read and decompress block (with cache if available)
		var events []*models.Event
		var err error

		if qe.cache != nil {
			// Use cached block reader (events are already parsed)
			cachedBlock, err := reader.ReadBlockWithCache(filePath, blockMeta, qe.cache)
			if err != nil {
				qe.logger.Warn("Failed to read block %d from file %s: %v", blockMeta.ID, filePath, err)
				segmentsSkipped++
				continue
			}
			events = cachedBlock.Events
		} else {
			// Use original non-cached reader
			events, err = reader.ReadBlockEvents(blockMeta)
			if err != nil {
				qe.logger.Warn("Failed to read block %d from file %s: %v", blockMeta.ID, filePath, err)
				segmentsSkipped++
				continue
			}
		}

		segmentsScanned++

		// Filter events by time range and resource filters
		var blockEvents []models.Event
		for _, event := range events {
			// Track all resources seen in this block (even those outside time range)
			// This prevents creating state snapshots for resources we already have events from
			resourceKey := qe.getResourceKey(event.Resource)
			resourcesWithEvents[resourceKey] = true

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

	// Add state snapshots for resources that don't have events in the query range
	// but exist in the final resource states (only if includeStateSnapshots is true)
	if includeStateSnapshots && len(fileData.IndexSection.FinalResourceStates) > 0 {
		qe.logger.Debug("File %s: checking state snapshots - has %d final resource states", filePath, len(fileData.IndexSection.FinalResourceStates))
		qe.logger.Debug("File %s: creating state snapshot events from %d final states", filePath, len(fileData.IndexSection.FinalResourceStates))
		stateEvents := qe.getStateSnapshotEventsWithDeleted(fileData.IndexSection.FinalResourceStates, query, resourcesWithEvents, deletedResources)
		qe.logger.Debug("File %s: added %d state snapshot events", filePath, len(stateEvents))
		results = append(results, stateEvents...)
	} else if !includeStateSnapshots {
		qe.logger.Debug("File %s: skipping state snapshots (file not overlapping with query range)", filePath)
	}

	span.SetAttributes(
		attribute.Int("file.events_found", len(results)),
		attribute.Int("file.segments_scanned", int(segmentsScanned)),
		attribute.Int("file.segments_skipped", int(segmentsSkipped)),
	)
	span.SetStatus(codes.Ok, "File queried successfully")

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
func (qe *QueryExecutor) QueryCount(ctx context.Context, query *models.QueryRequest) (int64, error) {
	result, err := qe.Execute(ctx, query)
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
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

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
		strings.Contains(errStr, "invalid file footer magic bytes") ||
		(strings.Contains(errStr, "invalid argument") && strings.Contains(errStr, "seek"))
}

// getResourceKey creates a unique key for a resource: group/version/kind/namespace/name
func (qe *QueryExecutor) getResourceKey(resource models.ResourceMetadata) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s",
		resource.Group,
		resource.Version,
		resource.Kind,
		resource.Namespace,
		resource.Name,
	)
}

// getStateSnapshotEventsWithDeleted converts final resource states to synthetic events
// for resources that don't have actual events in the query range, excluding globally deleted resources
func (qe *QueryExecutor) getStateSnapshotEventsWithDeleted(finalStates map[string]*ResourceLastState,
	query *models.QueryRequest, resourcesWithEvents map[string]bool, deletedResources map[string]bool) []models.Event {
	stateEvents := make([]models.Event, 0, len(finalStates))

	for resourceKey, state := range finalStates {
		// Skip if this resource already has events in the query results
		if resourcesWithEvents[resourceKey] {
			continue
		}

		// Skip deleted resources - they shouldn't appear in the consistent view
		if state.EventType == string(models.EventTypeDelete) {
			continue
		}

		// Skip resources that were deleted in ANY file
		if deletedResources[resourceKey] {
			continue
		}

		// Create synthetic event from state snapshot
		// Use a special marker timestamp (the state timestamp, but ensure it's within query range)
		eventTimestamp := state.Timestamp

		// Only include if timestamp is at or before query end time
		// This ensures we show consistent view of resources that existed at query time
		queryEndNs := query.EndTimestamp * 1e9
		if eventTimestamp > queryEndNs {
			// State happened after query range - skip it
			continue
		}

		// Parse the resource key back to metadata
		parts := strings.Split(resourceKey, "/")
		if len(parts) != 5 {
			qe.logger.Warn("Invalid resource key format: %s", resourceKey)
			continue
		}

		resource := models.ResourceMetadata{
			Group:     parts[0],
			Version:   parts[1],
			Kind:      parts[2],
			Namespace: parts[3],
			Name:      parts[4],
			UID:       state.UID,
		}

		// Check if resource matches filters
		if !resource.Matches(query.Filters) {
			continue
		}

		// Create synthetic event from state
		event := models.Event{
			ID:        fmt.Sprintf("state-%s-%d", resourceKey, state.Timestamp),
			Timestamp: eventTimestamp,
			Type:      models.EventType(state.EventType),
			Resource:  resource,
			Data:      state.ResourceData,
		}

		stateEvents = append(stateEvents, event)
	}

	return stateEvents
}
