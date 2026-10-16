package embeddedstore

import (
	"sort"

	"github.com/moolen/spectre/internal/models"
)

func (p *Projection) resourceTimelineEvents(uid string, startTimeNs, endTimeNs int64) []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record := p.resourcesByUID[uid]
	return resourceRecordTimelineEvents(record, startTimeNs, endTimeNs, 0)
}

// maxTimelineVersionsPerResource is the number of versions above which a timeline page
// thins a resource's history (see timelineVersionKept). Over long windows objects that
// report status periodically, such as Nodes, have thousands of versions, each sent with
// its full object.
const maxTimelineVersionsPerResource = 256

// resourceRecordTimelineEvents returns the record's versions in the window, preceded by
// the version current at its start. With maxVersions > 0 and more versions than that in
// the window, versions that repeat the previous status without a configuration change
// are left out.
func resourceRecordTimelineEvents(record *resourceRecord, startTimeNs, endTimeNs int64, maxVersions int) []models.Event {
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

	thin := maxVersions > 0 && endIdx-startIdx > maxVersions
	lastKept := startIdx - 1
	for i := startIdx; i < endIdx; i++ {
		if thin && !timelineVersionKept(record.versions, i, lastKept, endIdx) {
			continue
		}
		result = append(result, resourceVersionEvent(record.versions[i]))
		lastKept = i
	}

	if len(result) == 0 {
		return nil
	}

	return dedupeEventsByID(result)
}

// timelineVersionKept reports whether a thinned history keeps versions[i]: the first and
// last versions of the window, creates and deletes, configuration changes, and versions
// whose status differs from the last kept one. What is left out are status-only updates
// that do not change the displayed status, such as heartbeats; the diff between kept
// neighbours still covers them.
func timelineVersionKept(versions []resourceVersion, i, lastKept, endIdx int) bool {
	version := &versions[i]
	if lastKept < 0 || i == endIdx-1 || version.eventType() != models.EventTypeUpdate || version.configChanged() {
		return true
	}
	return version.statusCode != versions[lastKept].statusCode
}

// resourceRecordHasWindowEvents reports whether resourceRecordTimelineEvents would return
// anything for the window, without materialising events or reading payloads.
func resourceRecordHasWindowEvents(record *resourceRecord, startTimeNs, endTimeNs int64) bool {
	if record == nil || len(record.versions) == 0 || endTimeNs < startTimeNs {
		return false
	}
	versions := record.versions
	if versions[0].timestamp > endTimeNs {
		return false
	}
	startIdx := sort.Search(len(versions), func(i int) bool {
		return versions[i].timestamp >= startTimeNs
	})
	if startIdx < len(versions) && versions[startIdx].timestamp <= endTimeNs {
		return true
	}
	return startIdx > 0 && versions[startIdx-1].eventType() != models.EventTypeDelete
}

// resourceRecordWindowBounds returns the earliest and latest timestamps of the events
// resourceRecordTimelineEvents would return, and false when there are none.
func resourceRecordWindowBounds(record *resourceRecord, startTimeNs, endTimeNs int64) (int64, int64, bool) {
	if !resourceRecordHasWindowEvents(record, startTimeNs, endTimeNs) {
		return 0, 0, false
	}
	versions := record.versions
	startIdx := sort.Search(len(versions), func(i int) bool {
		return versions[i].timestamp >= startTimeNs
	})
	endIdx := sort.Search(len(versions), func(i int) bool {
		return versions[i].timestamp > endTimeNs
	})
	first, last := startIdx, endIdx-1
	if startIdx > 0 && versions[startIdx-1].eventType() != models.EventTypeDelete {
		first = startIdx - 1
	}
	if last < first {
		last = first
	}
	return versions[first].timestamp, versions[last].timestamp, true
}

func resourceRecordCurrentKey(record *resourceRecord) orderedResourceKey {
	if record == nil || len(record.versions) == 0 {
		return orderedResourceKey{}
	}
	latest := record.versions[len(record.versions)-1]
	return orderedResourceKey{
		kind:      latest.meta.Kind,
		namespace: latest.meta.Namespace,
		name:      latest.meta.Name,
		uid:       latest.meta.UID,
	}
}

func resourceRecordLatestMeta(record *resourceRecord) models.ResourceMetadata {
	if record == nil || len(record.versions) == 0 {
		return models.ResourceMetadata{}
	}
	return resourceMetadataFromIdentity(record.versions[len(record.versions)-1].identity())
}

func resourceRecordPreExistingEvent(record *resourceRecord) (models.Event, bool) {
	if record == nil {
		return models.Event{}, false
	}
	latest := record.latestVersion()
	if latest == nil || latest.eventType() == models.EventTypeDelete {
		return models.Event{}, false
	}
	event := resourceVersionEvent(*latest)
	event.PreExisting = true
	return event, true
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
		if deleted := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs); deleted != nil && deleted.eventType() == models.EventTypeDelete {
			if deletedCandidate == nil || deleted.timestamp > deletedCandidate.timestamp {
				deletedCandidate = deleted
			}
		}
	}
	return deletedCandidate
}

func (p *Projection) activeResourcesInNamespace(namespace string, timestampNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range uniqueRecords(p.graph.byNamespace[namespace]) {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.meta.Kind == "Event" || version.meta.Namespace != namespace {
			continue
		}
		result = append(result, version)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].meta.Kind != result[j].meta.Kind {
			return result[i].meta.Kind < result[j].meta.Kind
		}
		if result[i].meta.Name != result[j].meta.Name {
			return result[i].meta.Name < result[j].meta.Name
		}
		return result[i].meta.UID < result[j].meta.UID
	})

	return result
}
