package embeddedstore

import (
	"bytes"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

type QueryPlanner struct {
	projection *Projection
	hot        *hotStore
	segments   []*segmentReader

	mu                   sync.RWMutex
	segmentDimCache      map[string]map[segmentDimensionEntry]struct{}
	uidSegmentRoute      map[string][]*segmentReader
	uidRouteFallbackByID map[string]struct{}
	uidKeysBySegmentID   map[string][]string
	uidRoutePendingByID  map[string]struct{}
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
	return newQueryPlanner(projection, hot, segments, nil)
}

func newQueryPlanner(projection *Projection, hot *hotStore, segments []*segmentReader, previous *QueryPlanner) *QueryPlanner {
	planner := &QueryPlanner{
		projection:           projection,
		hot:                  hot,
		segments:             append([]*segmentReader(nil), segments...),
		segmentDimCache:      make(map[string]map[segmentDimensionEntry]struct{}),
		uidSegmentRoute:      make(map[string][]*segmentReader),
		uidRouteFallbackByID: make(map[string]struct{}),
		uidKeysBySegmentID:   make(map[string][]string),
		uidRoutePendingByID:  make(map[string]struct{}),
	}
	// The UID route only serves per-resource segment lookups, which the engine no longer
	// needs (timeline and analysis reads go through the projection's payload
	// references). Building it eagerly copied every UID of every segment on each flush,
	// so segments are left unrouted and such lookups consult each segment's index.
	for _, reader := range planner.segments {
		if reader != nil {
			planner.uidRouteFallbackByID[reader.ID()] = struct{}{}
		}
	}
	return planner
}

// clampToRetention raises a window start to the retention cutoff: segments that straddle
// the cutoff are kept whole and still hold expired records.
func (p *QueryPlanner) clampToRetention(startTimeNs int64) int64 {
	if p == nil || p.projection == nil {
		return startTimeNs
	}
	return max(startTimeNs, p.projection.retentionCutoffNs.Load())
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

	dedupedIDs := make([]string, 0, len(events))
	bestByID := make(map[string]models.Event, len(events))
	for i := range events {
		existing, ok := bestByID[events[i].ID]
		if !ok {
			dedupedIDs = append(dedupedIDs, events[i].ID)
			bestByID[events[i].ID] = events[i]
			continue
		}
		if compareDuplicateEventPreference(events[i], existing) < 0 {
			bestByID[events[i].ID] = events[i]
		}
	}

	deduped := make([]models.Event, 0, len(dedupedIDs))
	for i := range dedupedIDs {
		deduped = append(deduped, bestByID[dedupedIDs[i]])
	}

	return deduped
}

func compareDuplicateEventPreference(left, right models.Event) int {
	if cmp := compareEventOrder(left, right); cmp != 0 {
		return cmp
	}
	if left.PreExisting != right.PreExisting {
		if !left.PreExisting {
			return -1
		}
		return 1
	}
	if cmp := compareResourceMetadata(left.Resource, right.Resource); cmp != 0 {
		return cmp
	}
	if left.Type != right.Type {
		if left.Type < right.Type {
			return -1
		}
		return 1
	}
	if cmp := bytes.Compare(left.Data, right.Data); cmp != 0 {
		return cmp
	}
	if left.DataSize != right.DataSize {
		if left.DataSize < right.DataSize {
			return -1
		}
		return 1
	}
	if left.CompressedSize != right.CompressedSize {
		if left.CompressedSize < right.CompressedSize {
			return -1
		}
		return 1
	}
	return 0
}

func compareResourceMetadata(left, right models.ResourceMetadata) int {
	if left.Group != right.Group {
		if left.Group < right.Group {
			return -1
		}
		return 1
	}
	if left.Version != right.Version {
		if left.Version < right.Version {
			return -1
		}
		return 1
	}
	if left.Kind != right.Kind {
		if left.Kind < right.Kind {
			return -1
		}
		return 1
	}
	if left.Namespace != right.Namespace {
		if left.Namespace < right.Namespace {
			return -1
		}
		return 1
	}
	if left.Name != right.Name {
		if left.Name < right.Name {
			return -1
		}
		return 1
	}
	if left.UID != right.UID {
		if left.UID < right.UID {
			return -1
		}
		return 1
	}
	if left.InvolvedObjectUID < right.InvolvedObjectUID {
		return -1
	}
	if left.InvolvedObjectUID > right.InvolvedObjectUID {
		return 1
	}
	return 0
}
