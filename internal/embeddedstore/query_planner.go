package embeddedstore

import (
	"sync"

	"github.com/moolen/spectre/internal/models"
)

type QueryPlanner struct {
	projection *Projection
	hot        *hotStore
	segments   []*segmentReader

	mu              sync.RWMutex
	segmentDimCache map[string]map[segmentDimensionEntry]struct{}
}

type queryPlanStats struct {
	projectionUsed   bool
	hotUsed          bool
	coldUsed         bool
	relevantSegments int
	scannedSegments  int
	uidDiskLookups   int
	hotScans         int
}

func NewQueryPlanner(projection *Projection, hot *hotStore, segments []*segmentReader) *QueryPlanner {
	return newQueryPlanner(projection, hot, segments)
}

func newQueryPlanner(projection *Projection, hot *hotStore, segments []*segmentReader) *QueryPlanner {
	return &QueryPlanner{
		projection:      projection,
		hot:             hot,
		segments:        append([]*segmentReader(nil), segments...),
		segmentDimCache: make(map[string]map[segmentDimensionEntry]struct{}),
	}
}

func (s queryPlanStats) merge(other queryPlanStats) queryPlanStats {
	return queryPlanStats{
		projectionUsed:   s.projectionUsed || other.projectionUsed,
		hotUsed:          s.hotUsed || other.hotUsed,
		coldUsed:         s.coldUsed || other.coldUsed,
		relevantSegments: s.relevantSegments + other.relevantSegments,
		scannedSegments:  s.scannedSegments + other.scannedSegments,
		uidDiskLookups:   s.uidDiskLookups + other.uidDiskLookups,
		hotScans:         s.hotScans + other.hotScans,
	}
}

func (s queryPlanStats) storeMix() string {
	switch {
	case s.projectionUsed && !s.hotUsed && !s.coldUsed:
		return storeMixProjectionOnly
	case s.hotUsed && s.coldUsed:
		return storeMixMixed
	case s.hotUsed:
		return storeMixHotOnly
	case s.coldUsed:
		return storeMixColdOnly
	default:
		return storeMixProjectionOnly
	}
}

func resourceEventsInWindow(events []models.Event, startTimeNs, endTimeNs int64) []models.Event {
	if len(events) == 0 {
		return nil
	}

	var inRange []models.Event
	var lastBefore models.Event
	var hasLastBefore bool

	for i := range events {
		event := events[i]
		if event.Timestamp < startTimeNs {
			lastBefore = cloneEvent(event)
			hasLastBefore = true
			continue
		}
		if event.Timestamp > endTimeNs {
			break
		}
		inRange = append(inRange, cloneEvent(event))
	}

	var result []models.Event
	if hasLastBefore && lastBefore.Type != models.EventTypeDelete {
		lastBefore.PreExisting = true
		result = append(result, lastBefore)
	}

	if len(inRange) == 0 && len(result) == 0 {
		return nil
	}

	result = append(result, inRange...)
	return result
}

func dedupeEventsByID(events []models.Event) []models.Event {
	if len(events) <= 1 {
		return events
	}

	deduped := make([]models.Event, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for i := range events {
		if _, ok := seen[events[i].ID]; ok {
			continue
		}
		seen[events[i].ID] = struct{}{}
		deduped = append(deduped, events[i])
	}

	return deduped
}
