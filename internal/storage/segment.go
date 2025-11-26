package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// Segment represents a unit of compressed event data
type Segment struct {
	ID                 int32
	startTimestamp     int64
	endTimestamp       int64
	events             []*models.Event
	uncompressedData   []byte
	compressedData     []byte
	metadata           models.SegmentMetadata
	logger             *logging.Logger
	compressor         *Compressor
	maxSize            int64
	namespaceSet       map[string]bool
	kindSet            map[string]bool
	resourceSummary    []models.ResourceMetadata
	outOfOrderBuffer   []*models.Event // Buffer for out-of-order events
	lastEventTime      int64            // Track last event time for reordering
	reorderingWindowMs int64            // Time window (ms) to wait for out-of-order events
}

// NewSegment creates a new segment
func NewSegment(id int32, compressor *Compressor, maxSize int64) *Segment {
	return &Segment{
		ID:                 id,
		events:             make([]*models.Event, 0),
		logger:             logging.GetLogger("segment"),
		compressor:         compressor,
		maxSize:            maxSize,
		namespaceSet:       make(map[string]bool),
		kindSet:            make(map[string]bool),
		resourceSummary:    make([]models.ResourceMetadata, 0),
		outOfOrderBuffer:   make([]*models.Event, 0),
		reorderingWindowMs: 5000, // Default 5-second reordering window
	}
}

// AddEvent adds an event to the segment with out-of-order handling
func (s *Segment) AddEvent(event *models.Event) error {
	// Validate event
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	// Handle out-of-order events with buffering and reordering
	if len(s.events) > 0 && event.Timestamp < s.lastEventTime {
		// Event is out of order - check if it's within reordering window
		timeDiff := s.lastEventTime - event.Timestamp
		if timeDiff <= s.reorderingWindowMs {
			// Within reordering window, buffer the event for later insertion
			s.outOfOrderBuffer = append(s.outOfOrderBuffer, event)
			s.logger.Debug("Buffered out-of-order event for segment %d (time_diff=%dms)", s.ID, timeDiff)
			return nil
		}
		// Outside reordering window - log but still add the event
		s.logger.Warn("Event for segment %d is severely out-of-order (time_diff=%dms), adding anyway", s.ID, timeDiff)
	}

	// Add the event to the main events list
	s.events = append(s.events, event)
	s.lastEventTime = event.Timestamp

	// Flush buffered out-of-order events that are now in order
	s.flushOrderedBufferedEvents()

	// Update timestamps
	if len(s.events) == 1 {
		s.startTimestamp = event.Timestamp
		s.endTimestamp = event.Timestamp
	} else {
		if event.Timestamp < s.startTimestamp {
			s.startTimestamp = event.Timestamp
		}
		if event.Timestamp > s.endTimestamp {
			s.endTimestamp = event.Timestamp
		}
	}

	// Update metadata
	s.updateMetadata(event)

	return nil
}

// flushOrderedBufferedEvents flushes buffered out-of-order events that are now in chronological order
func (s *Segment) flushOrderedBufferedEvents() {
	if len(s.outOfOrderBuffer) == 0 {
		return
	}

	// Sort buffered events by timestamp
	for i := 0; i < len(s.outOfOrderBuffer); i++ {
		for j := i + 1; j < len(s.outOfOrderBuffer); j++ {
			if s.outOfOrderBuffer[j].Timestamp < s.outOfOrderBuffer[i].Timestamp {
				s.outOfOrderBuffer[i], s.outOfOrderBuffer[j] = s.outOfOrderBuffer[j], s.outOfOrderBuffer[i]
			}
		}
	}

	// Add buffered events that are now in order
	remainingBuffer := make([]*models.Event, 0)
	for _, bufferedEvent := range s.outOfOrderBuffer {
		if bufferedEvent.Timestamp <= s.lastEventTime {
			// This event is now in order, add it to the main events list
			// Find the correct position to insert
			inserted := false
			for i := len(s.events) - 1; i >= 0; i-- {
				if s.events[i].Timestamp <= bufferedEvent.Timestamp {
					// Insert after this event
					s.events = append(s.events[:i+1], append([]*models.Event{bufferedEvent}, s.events[i+1:]...)...)
					s.updateMetadata(bufferedEvent)
					inserted = true
					s.logger.Debug("Flushed buffered event for segment %d into main event stream", s.ID)
					break
				}
			}
			if !inserted {
				// Insert at beginning
				s.events = append([]*models.Event{bufferedEvent}, s.events...)
				s.updateMetadata(bufferedEvent)
			}
		} else {
			// Still out of order, keep in buffer
			remainingBuffer = append(remainingBuffer, bufferedEvent)
		}
	}

	s.outOfOrderBuffer = remainingBuffer
}

// SetReorderingWindow sets the time window (in milliseconds) for buffering out-of-order events
func (s *Segment) SetReorderingWindow(windowMs int64) {
	s.reorderingWindowMs = windowMs
	s.logger.Debug("Set reordering window for segment %d to %dms", s.ID, windowMs)
}

// GetBufferedEventCount returns the number of events waiting in the out-of-order buffer
func (s *Segment) GetBufferedEventCount() int32 {
	return int32(len(s.outOfOrderBuffer))
}

// FlushBufferedEvents forces all buffered out-of-order events into the main stream
func (s *Segment) FlushBufferedEvents() {
	if len(s.outOfOrderBuffer) == 0 {
		return
	}

	s.logger.Debug("Force-flushing %d buffered events for segment %d", len(s.outOfOrderBuffer), s.ID)

	// Sort all buffered events by timestamp
	for i := 0; i < len(s.outOfOrderBuffer); i++ {
		for j := i + 1; j < len(s.outOfOrderBuffer); j++ {
			if s.outOfOrderBuffer[j].Timestamp < s.outOfOrderBuffer[i].Timestamp {
				s.outOfOrderBuffer[i], s.outOfOrderBuffer[j] = s.outOfOrderBuffer[j], s.outOfOrderBuffer[i]
			}
		}
	}

	// Merge buffered events into main events list in sorted order
	mergedEvents := make([]*models.Event, 0, len(s.events)+len(s.outOfOrderBuffer))
	i, j := 0, 0

	for i < len(s.events) && j < len(s.outOfOrderBuffer) {
		if s.events[i].Timestamp <= s.outOfOrderBuffer[j].Timestamp {
			mergedEvents = append(mergedEvents, s.events[i])
			i++
		} else {
			mergedEvents = append(mergedEvents, s.outOfOrderBuffer[j])
			j++
		}
	}

	// Append remaining events
	mergedEvents = append(mergedEvents, s.events[i:]...)
	mergedEvents = append(mergedEvents, s.outOfOrderBuffer[j:]...)

	s.events = mergedEvents
	s.outOfOrderBuffer = make([]*models.Event, 0)

	s.logger.Info("Flushed all buffered events into segment %d", s.ID)
}

// updateMetadata updates segment metadata with the new event
func (s *Segment) updateMetadata(event *models.Event) {
	s.namespaceSet[event.Resource.Namespace] = true
	s.kindSet[event.Resource.Kind] = true

	// Check if this resource is already in the summary
	found := false
	for _, r := range s.resourceSummary {
		if r.Group == event.Resource.Group &&
			r.Version == event.Resource.Version &&
			r.Kind == event.Resource.Kind &&
			r.Namespace == event.Resource.Namespace {
			found = true
			break
		}
	}

	if !found {
		s.resourceSummary = append(s.resourceSummary, event.Resource)
	}
}

// Finalize finalizes the segment by compressing the data
func (s *Segment) Finalize() error {
	// Flush any remaining buffered out-of-order events before finalization
	if len(s.outOfOrderBuffer) > 0 {
		s.FlushBufferedEvents()
	}

	if len(s.events) == 0 {
		return fmt.Errorf("cannot finalize empty segment")
	}

	// Serialize events to JSON
	var buf bytes.Buffer
	for _, event := range s.events {
		jsonData, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		buf.Write(jsonData)
		buf.WriteString("\n") // Add newline for easy parsing
	}

	s.uncompressedData = buf.Bytes()

	// Compress the data
	compressed, err := s.compressor.Compress(s.uncompressedData)
	if err != nil {
		return fmt.Errorf("failed to compress segment data: %w", err)
	}

	s.compressedData = compressed

	// Build metadata
	s.metadata = models.SegmentMetadata{
		ResourceSummary:      s.resourceSummary,
		MinTimestamp:         s.startTimestamp,
		MaxTimestamp:         s.endTimestamp,
		NamespaceSet:         s.namespaceSet,
		KindSet:              s.kindSet,
		CompressionAlgorithm: "gzip",
	}

	s.logger.Debug("Segment %d finalized: events=%d, buffered=%d (flushed), uncompressed=%d, compressed=%d, ratio=%.2f",
		s.ID, len(s.events), len(s.outOfOrderBuffer), len(s.uncompressedData), len(s.compressedData),
		s.getCompressionRatio())

	return nil
}

// GetEventCount returns the number of events in the segment
func (s *Segment) GetEventCount() int32 {
	return int32(len(s.events))
}

// GetEvents returns a copy of the events in the segment (thread-safe read)
func (s *Segment) GetEvents() []*models.Event {
	// Return a copy to avoid race conditions
	events := make([]*models.Event, len(s.events))
	copy(events, s.events)
	return events
}

// GetUncompressedSize returns the uncompressed size in bytes
func (s *Segment) GetUncompressedSize() int64 {
	return int64(len(s.uncompressedData))
}

// GetCompressedSize returns the compressed size in bytes
func (s *Segment) GetCompressedSize() int64 {
	return int64(len(s.compressedData))
}

// getCompressionRatio returns the compression ratio
func (s *Segment) getCompressionRatio() float64 {
	if len(s.uncompressedData) == 0 {
		return 0.0
	}
	return float64(len(s.compressedData)) / float64(len(s.uncompressedData))
}

// GetCompressedData returns the compressed event data
func (s *Segment) GetCompressedData() []byte {
	return s.compressedData
}

// GetDecompressedEvents decompresses and returns events from the segment
func (s *Segment) GetDecompressedEvents() ([]models.Event, error) {
	// Decompress the data
	decompressed, err := s.compressor.Decompress(s.compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress segment data: %w", err)
	}

	// Parse JSON-encoded events
	var events []models.Event
	lines := bytes.Split(decompressed, []byte("\n"))

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			s.logger.Warn("Failed to unmarshal event: %v", err)
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

// IsReady checks if the segment is ready to be finalized
func (s *Segment) IsReady() bool {
	return len(s.events) > 0 && int64(len(s.uncompressedData)) >= s.maxSize
}

// GetMetadata returns the segment metadata
func (s *Segment) GetMetadata() models.SegmentMetadata {
	return s.metadata
}

// ToStorageSegment converts the segment to a models.StorageSegment
func (s *Segment) ToStorageSegment(offset, length int64) models.StorageSegment {
	return models.StorageSegment{
		ID:                 s.ID,
		StartTimestamp:     s.startTimestamp,
		EndTimestamp:       s.endTimestamp,
		EventCount:         s.GetEventCount(),
		UncompressedSize:   s.GetUncompressedSize(),
		CompressedSize:     s.GetCompressedSize(),
		Offset:             offset,
		Length:             length,
		Metadata:           s.metadata,
	}
}

// FilterEvents returns events matching the specified filters
func (s *Segment) FilterEvents(filters models.QueryFilters) ([]*models.Event, error) {
	// Get decompressed events
	events, err := s.GetDecompressedEvents()
	if err != nil {
		return nil, err
	}

	// Filter events
	var filtered []*models.Event
	for i := range events {
		if filters.Matches(events[i].Resource) {
			filtered = append(filtered, &events[i])
		}
	}

	return filtered, nil
}

// MatchesFilters checks if the segment contains events matching the filters
func (s *Segment) MatchesFilters(filters models.QueryFilters) bool {
	return s.metadata.MatchesFilters(filters)
}

// IsInTimeRange checks if the segment overlaps with the specified time range
func (s *Segment) IsInTimeRange(startTime, endTime int64) bool {
	return s.startTimestamp <= endTime && s.endTimestamp >= startTime
}

// WriteToBuffer writes the segment data to a buffer
func (s *Segment) WriteToBuffer() *bytes.Buffer {
	buf := new(bytes.Buffer)

	// Write segment ID
	binary.Write(buf, binary.LittleEndian, s.ID)

	// Write timestamps
	binary.Write(buf, binary.LittleEndian, s.startTimestamp)
	binary.Write(buf, binary.LittleEndian, s.endTimestamp)

	// Write event count
	binary.Write(buf, binary.LittleEndian, int32(len(s.events)))

	// Write sizes
	binary.Write(buf, binary.LittleEndian, int64(len(s.uncompressedData)))
	binary.Write(buf, binary.LittleEndian, int64(len(s.compressedData)))

	// Write compressed data
	buf.Write(s.compressedData)

	return buf
}
