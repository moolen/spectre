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
	events                []models.Event
	recentByUID           map[string]*RecentResourceLog
	recentAssociatedByUID map[string]*RecentResourceLog
	namespaceCounts       map[string]int
	kindCounts            map[string]int
	groupCounts           map[string]int
	versionCounts         map[string]int
}

func newHotStore(config HotStoreConfig) *hotStore {
	return &hotStore{
		config:                config,
		recentByUID:           make(map[string]*RecentResourceLog),
		recentAssociatedByUID: make(map[string]*RecentResourceLog),
		namespaceCounts:       make(map[string]int),
		kindCounts:            make(map[string]int),
		groupCounts:           make(map[string]int),
		versionCounts:         make(map[string]int),
	}
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

	return HotFlushBatch{
		Events: cloneEvents(s.events[:limit]),
	}
}

func (s *hotStore) CommitFlushedBatch(batch HotFlushBatch) int {
	if len(batch.Events) == 0 {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for i := range batch.Events {
		if s.removeEventLocked(batch.Events[i].ID) {
			removed++
		}
	}
	return removed
}

func (s *hotStore) appendOneLocked(event models.Event) {
	s.events = insertEventSorted(s.events, event)
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

	s.enforceGlobalBoundLocked()
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
	log.Events = insertEventSorted(log.Events, event)
}

func (s *hotStore) enforceLogBoundLocked(logs map[string]*RecentResourceLog, uid string) {
	if s.config.MaxResourceVersions <= 0 || uid == "" {
		return
	}

	for {
		log := logs[uid]
		if log == nil || len(log.Events) <= s.config.MaxResourceVersions {
			return
		}
		s.removeEventLocked(log.Events[0].ID)
	}
}

func (s *hotStore) enforceGlobalBoundLocked() {
	if s.config.MaxEvents <= 0 {
		return
	}
	for len(s.events) > s.config.MaxEvents {
		if !s.removeEventLocked(s.events[0].ID) {
			return
		}
	}
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
	if event.Resource.Kind == "Event" {
		s.removeFromLogLocked(s.recentAssociatedByUID, event.Resource.InvolvedObjectUID, eventID)
	} else {
		s.removeFromLogLocked(s.recentByUID, event.Resource.UID, eventID)
	}
	s.decrementMetadataLocked(event.Resource)
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
