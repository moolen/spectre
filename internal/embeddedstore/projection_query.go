package embeddedstore

import (
	"sort"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (p *Projection) resourceTimelineEvents(uid string, startTimeNs, endTimeNs int64) []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record := p.resourcesByUID[uid]
	return resourceRecordTimelineEvents(record, startTimeNs, endTimeNs)
}

func resourceRecordTimelineEvents(record *resourceRecord, startTimeNs, endTimeNs int64) []models.Event {
	if record == nil || len(record.versions) == 0 || endTimeNs < startTimeNs {
		return nil
	}

	startIdx := sort.Search(len(record.versions), func(i int) bool {
		return record.versions[i].timestamp >= startTimeNs
	})
	endIdx := sort.Search(len(record.versions), func(i int) bool {
		return record.versions[i].timestamp > endTimeNs
	})

	result := make([]models.Event, 0, max(1, endIdx-startIdx+1))
	if startIdx > 0 {
		lastBefore := resourceVersionEvent(record.versions[startIdx-1])
		if lastBefore.Type != models.EventTypeDelete {
			lastBefore.PreExisting = true
			result = append(result, lastBefore)
		}
	}

	for i := startIdx; i < endIdx; i++ {
		result = append(result, resourceVersionEvent(record.versions[i]))
	}

	if len(result) == 0 {
		return nil
	}

	return dedupeEventsByID(result)
}

func resourceRecordCurrentKey(record *resourceRecord) orderedResourceKey {
	if record == nil || len(record.versions) == 0 {
		return orderedResourceKey{}
	}
	latest := record.versions[len(record.versions)-1]
	return orderedResourceKey{
		kind:      latest.identity.Kind,
		namespace: latest.identity.Namespace,
		name:      latest.identity.Name,
		uid:       latest.identity.UID,
	}
}

func resourceRecordLatestMeta(record *resourceRecord) models.ResourceMetadata {
	if record == nil || len(record.versions) == 0 {
		return models.ResourceMetadata{}
	}
	return resourceMetadataFromIdentity(record.versions[len(record.versions)-1].identity)
}

func resourceRecordPreExistingEvent(record *resourceRecord) (models.Event, bool) {
	if record == nil {
		return models.Event{}, false
	}
	latest := record.latestVersion()
	if latest == nil || latest.eventType == models.EventTypeDelete {
		return models.Event{}, false
	}
	event := resourceVersionEvent(*latest)
	event.PreExisting = true
	return event, true
}

func (p *Projection) recentWindowChangedUIDs(startTimeNs int64) (map[string]struct{}, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.recentWindowChangedUIDsLocked(startTimeNs)
}

func (p *Projection) recentWindowChangedUIDsLocked(startTimeNs int64) (map[string]struct{}, bool) {
	if p == nil || p.maxTimestampNs <= 0 || len(p.recentResourceChanges) == 0 {
		return nil, false
	}

	if p.retentionWindowNs > 0 {
		retentionStart := time.Now().UTC().Add(-time.Duration(p.retentionWindowNs)).UnixNano()
		if startTimeNs < retentionStart {
			return nil, false
		}
	}

	idx := sort.Search(len(p.recentResourceChanges), func(i int) bool {
		return p.recentResourceChanges[i].timestamp >= startTimeNs
	})
	if idx >= len(p.recentResourceChanges) {
		return nil, false
	}
	changedUIDs := make(map[string]struct{})
	for i := idx; i < len(p.recentResourceChanges); i++ {
		changedUIDs[p.recentResourceChanges[i].uid] = struct{}{}
	}
	return changedUIDs, true
}

func (p *Projection) rebuildRecord(uid string) {
	p.rebuildRecordFromEvents(uid, p.eventsByResourceUID[uid])
}

func (p *Projection) resolveByName(namespace, kind, name string, failureTimestampNs, windowStartNs int64) *resourceVersion {
	records := p.resourcesByKey[resourceKey{namespace: namespace, kind: kind, name: name}]
	var deletedCandidate *resourceVersion
	for _, record := range records {
		if active := record.activeVersionAt(failureTimestampNs); active != nil {
			return active
		}
		if windowStartNs == 0 {
			continue
		}
		if deleted := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs); deleted != nil && deleted.eventType == models.EventTypeDelete {
			if deletedCandidate == nil || deleted.timestamp > deletedCandidate.timestamp {
				deletedCandidate = deleted
			}
		}
	}
	return deletedCandidate
}

func (p *Projection) activeResourcesInNamespace(namespace string, timestampNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range p.resourcesByUID {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.identity.Kind == "Event" || version.identity.Namespace != namespace {
			continue
		}
		result = append(result, version)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].identity.Kind != result[j].identity.Kind {
			return result[i].identity.Kind < result[j].identity.Kind
		}
		if result[i].identity.Name != result[j].identity.Name {
			return result[i].identity.Name < result[j].identity.Name
		}
		return result[i].identity.UID < result[j].identity.UID
	})

	return result
}
