package embeddedstore

import (
	"sort"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

type HotStoreConfig struct {
	MaxEvents           int
	MaxResourceVersions int
}

type RecentResourceLog struct {
	UID    string
	Events []models.Event
}

type HotFlushBatch struct {
	Events []models.Event
}

type hotStore struct {
	mu sync.RWMutex

	config                HotStoreConfig
	metrics               *Metrics
	events                []models.Event
	sizeBytes             int64
	recentByUID           map[string]*RecentResourceLog
	recentAssociatedByUID map[string]*RecentResourceLog
	namespaceCounts       map[string]int
	kindCounts            map[string]int
	groupCounts           map[string]int
	versionCounts         map[string]int
}

func newHotStore(config HotStoreConfig, metrics *Metrics) *hotStore {
	store := &hotStore{
		config:                config,
		metrics:               metrics,
		recentByUID:           make(map[string]*RecentResourceLog),
		recentAssociatedByUID: make(map[string]*RecentResourceLog),
		namespaceCounts:       make(map[string]int),
		kindCounts:            make(map[string]int),
		groupCounts:           make(map[string]int),
		versionCounts:         make(map[string]int),
	}
	store.updateHotEventsLocked()
	return store
}

func (s *hotStore) Append(events []models.Event) {
	if len(events) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range events {
		s.appendOneLocked(cloneEvent(events[i]))
	}
	s.updateHotEventsLocked()
}

func (s *hotStore) ScanTimeRange(startTimestampNs, endTimestampNs int64) []models.Event {
	if endTimestampNs < startTimestampNs {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.events) == 0 {
		return nil
	}

	startIdx := sort.Search(len(s.events), func(i int) bool {
		return s.events[i].Timestamp >= startTimestampNs
	})
	endIdx := sort.Search(len(s.events), func(i int) bool {
		return s.events[i].Timestamp > endTimestampNs
	})
	if startIdx >= endIdx {
		return nil
	}
	return cloneEvents(s.events[startIdx:endIdx])
}

func (s *hotStore) RecentEventsByUID(uid string) []models.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log := s.recentByUID[uid]
	if log == nil {
		return nil
	}
	return cloneEvents(log.Events)
}

func (s *hotStore) RecentAssociatedEventsByUID(uid string) []models.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log := s.recentAssociatedByUID[uid]
	if log == nil {
		return nil
	}
	return cloneEvents(log.Events)
}

func (s *hotStore) DistinctMetadata() (namespaces, kinds, groups, versions []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespaces = mapKeysSorted(s.namespaceCounts)
	kinds = mapKeysSorted(s.kindCounts)
	groups = mapKeysSorted(s.groupCounts)
	versions = mapKeysSorted(s.versionCounts)
	return namespaces, kinds, groups, versions
}

func (s *hotStore) ExtractFlushBatch(maxEvents int) HotFlushBatch {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.events) == 0 {
		return HotFlushBatch{}
	}

	limit := len(s.events)
	if maxEvents > 0 && maxEvents < limit {
		limit = maxEvents
	}

	// Stored events are never mutated in place, so the batch can share payloads.
	return HotFlushBatch{
		Events: append([]models.Event(nil), s.events[:limit]...),
	}
}

// Len returns the number of unflushed events.
func (s *hotStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// SizeBytes returns the estimated serialized size of the unflushed events.
func (s *hotStore) SizeBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sizeBytes
}

func (s *hotStore) CommitFlushedBatch(batch HotFlushBatch) int {
	if len(batch.Events) == 0 {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Flushes run under the engine lock, so the batch is normally still the prefix of
	// the ordered event list and can be dropped in one step.
	if len(batch.Events) <= len(s.events) {
		prefix := true
		for i := range batch.Events {
			if s.events[i].ID != batch.Events[i].ID {
				prefix = false
				break
			}
		}
		if prefix {
			for i := range batch.Events {
				s.forgetEventLocked(s.events[i])
			}
			remaining := len(s.events) - len(batch.Events)
			copy(s.events, s.events[len(batch.Events):])
			clear(s.events[remaining:])
			s.events = s.events[:remaining]
			s.updateHotEventsLocked()
			return len(batch.Events)
		}
	}

	removed := 0
	for i := range batch.Events {
		if s.removeEventLocked(batch.Events[i].ID) {
			removed++
		}
	}
	s.updateHotEventsLocked()
	return removed
}

// forgetEventLocked drops an event's index entries and size without touching s.events.
func (s *hotStore) forgetEventLocked(event models.Event) {
	if event.Resource.Kind == "Event" {
		s.removeFromLogLocked(s.recentAssociatedByUID, event.Resource.InvolvedObjectUID, event.ID)
	} else {
		s.removeFromLogLocked(s.recentByUID, event.Resource.UID, event.ID)
	}
	s.decrementMetadataLocked(event.Resource)
	s.sizeBytes -= estimatedEventSizeBytes(&event)
}

func (s *hotStore) appendOneLocked(event models.Event) {
	s.events = appendOrderedEvent(s.events, event)
	s.sizeBytes += estimatedEventSizeBytes(&event)
	s.incrementMetadataLocked(event.Resource)

	if event.Resource.Kind == "Event" {
		if event.Resource.InvolvedObjectUID != "" {
			s.appendToLogLocked(s.recentAssociatedByUID, event.Resource.InvolvedObjectUID, event)
			s.enforceLogBoundLocked(s.recentAssociatedByUID, event.Resource.InvolvedObjectUID)
		}
	} else if event.Resource.UID != "" {
		s.appendToLogLocked(s.recentByUID, event.Resource.UID, event)
		s.enforceLogBoundLocked(s.recentByUID, event.Resource.UID)
	}
}

func (s *hotStore) appendToLogLocked(logs map[string]*RecentResourceLog, uid string, event models.Event) {
	if uid == "" {
		return
	}

	log := logs[uid]
	if log == nil {
		log = &RecentResourceLog{UID: uid}
		logs[uid] = log
	}
	log.Events = appendOrderedEvent(log.Events, event)
}

func (s *hotStore) enforceLogBoundLocked(logs map[string]*RecentResourceLog, uid string) {
	if s.config.MaxResourceVersions <= 0 || uid == "" {
		return
	}

	// Only the per-resource recent log is bounded. Every event stays queued until it is
	// flushed to a segment; the engine flushes once MaxEvents or the size budget is hit.
	log := logs[uid]
	if log == nil || len(log.Events) <= s.config.MaxResourceVersions {
		return
	}
	trimmed := len(log.Events) - s.config.MaxResourceVersions
	log.Events = append(log.Events[:0:0], log.Events[trimmed:]...)
	if s.metrics != nil {
		s.metrics.RecordHotEvictions("uid", trimmed)
	}
}

func (s *hotStore) updateHotEventsLocked() {
	if s.metrics == nil {
		return
	}
	s.metrics.SetHotEvents(len(s.events))
}

func (s *hotStore) removeEventLocked(eventID string) bool {
	if eventID == "" {
		return false
	}

	idx := -1
	var event models.Event
	for i := range s.events {
		if s.events[i].ID == eventID {
			idx = i
			event = s.events[i]
			break
		}
	}
	if idx < 0 {
		return false
	}

	s.events = append(s.events[:idx], s.events[idx+1:]...)
	s.forgetEventLocked(event)
	return true
}

func (s *hotStore) removeFromLogLocked(logs map[string]*RecentResourceLog, uid, eventID string) {
	if uid == "" {
		return
	}

	log := logs[uid]
	if log == nil {
		return
	}

	for i := range log.Events {
		if log.Events[i].ID != eventID {
			continue
		}
		log.Events = append(log.Events[:i], log.Events[i+1:]...)
		if len(log.Events) == 0 {
			delete(logs, uid)
		}
		return
	}
}

func (s *hotStore) incrementMetadataLocked(meta models.ResourceMetadata) {
	incrementCountIfPresent(s.namespaceCounts, meta.Namespace)
	incrementCountIfPresent(s.kindCounts, meta.Kind)
	incrementCountIfPresent(s.groupCounts, meta.Group)
	incrementCountIfPresent(s.versionCounts, meta.Version)
}

func (s *hotStore) decrementMetadataLocked(meta models.ResourceMetadata) {
	decrementCountIfPresent(s.namespaceCounts, meta.Namespace)
	decrementCountIfPresent(s.kindCounts, meta.Kind)
	decrementCountIfPresent(s.groupCounts, meta.Group)
	decrementCountIfPresent(s.versionCounts, meta.Version)
}

func incrementCountIfPresent(counts map[string]int, key string) {
	if key == "" {
		return
	}
	counts[key]++
}

func decrementCountIfPresent(counts map[string]int, key string) {
	if key == "" {
		return
	}
	count := counts[key]
	if count <= 1 {
		delete(counts, key)
		return
	}
	counts[key] = count - 1
}

func mapKeysSorted(counts map[string]int) []string {
	if len(counts) == 0 {
		return nil
	}

	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func appendOrderedEvent(events []models.Event, event models.Event) []models.Event {
	if len(events) == 0 || compareEventOrder(events[len(events)-1], event) <= 0 {
		return append(events, event)
	}
	return insertEventSorted(events, event)
}
