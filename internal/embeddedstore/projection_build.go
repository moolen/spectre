package embeddedstore

import (
	"sort"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
)

func BuildProjection(events []models.Event) (*Projection, error) {
	projection := NewProjection()

	sortedEvents := cloneEvents(events)
	sort.SliceStable(sortedEvents, func(i, j int) bool {
		return compareEventOrder(sortedEvents[i], sortedEvents[j]) < 0
	})

	projection.events = sortedEvents
	for i := range sortedEvents {
		projection.applyEventToIndexes(sortedEvents[i])
	}
	projection.finalizeReplayBuild()

	return projection, nil
}

func (p *Projection) appendReplayEvent(event models.Event) {
	cloned := cloneEvent(event)
	if p.retainHistoricalEventArrays {
		p.events = append(p.events, cloned)
	}
	p.applyEventToIndexes(cloned)
}

func (p *Projection) finalizeReplayBuild() {
	sort.Slice(p.orderedResources, func(i, j int) bool {
		return compareOrderedResourceKey(p.orderedResources[i], p.orderedResources[j]) < 0
	})
	sort.Slice(p.activeOrderedResources, func(i, j int) bool {
		return compareOrderedResourceKey(p.activeOrderedResources[i], p.activeOrderedResources[j]) < 0
	})

	if p.retainHistoricalEventArrays {
		for uid := range p.resourcesByUID {
			p.rebuildRecord(uid)
		}
	}
	for involvedUID := range p.k8sEventsByInvolvedUID {
		sort.Slice(p.k8sEventsByInvolvedUID[involvedUID], func(i, j int) bool {
			left := p.k8sEventsByInvolvedUID[involvedUID][i]
			right := p.k8sEventsByInvolvedUID[involvedUID][j]
			if left.Timestamp.Equal(right.Timestamp) {
				return left.EventID > right.EventID
			}
			return left.Timestamp.After(right.Timestamp)
		})
	}

	if !p.retainHistoricalEventArrays {
		p.events = nil
		clear(p.eventsByResourceUID)
		clear(p.k8sRawEventsByInvolvedUID)
	}
	p.pruneRecentResourceChangesLocked()
}

func (p *Projection) Apply(event models.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cloned := cloneEvent(event)
	if p.retainHistoricalEventArrays {
		p.events = appendOrInsertEventSorted(p.events, cloned)
	}

	if cloned.Resource.Kind == "Event" {
		p.applyK8sEvent(cloned)
		return nil
	}
	if cloned.Resource.UID == "" {
		return nil
	}

	p.ensureResourceRecord(cloned)

	uid := cloned.Resource.UID
	if p.retainHistoricalEventArrays {
		p.eventsByResourceUID[uid] = appendOrInsertEventSorted(p.eventsByResourceUID[uid], cloned)
	}

	record := p.resourcesByUID[uid]
	if record == nil {
		return nil
	}

	if canAppendResourceVersion(record, cloned) {
		p.appendResourceVersion(record, cloned)
		p.finishAppliedResourceEvent(uid, cloned, record, cloned.Resource)
		return nil
	}

	history := p.resourceEventsForUID(uid)
	history = insertEventSorted(history, cloned)
	if p.retainHistoricalEventArrays {
		p.eventsByResourceUID[uid] = history
	}
	p.rebuildRecordFromEvents(uid, history)
	p.finishAppliedResourceEvent(uid, cloned, p.resourcesByUID[uid], latestResourceMeta(history))

	return nil
}

func (p *Projection) ApplyReplayEvent(event models.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cloned := cloneEvent(event)
	if p.retainHistoricalEventArrays {
		p.events = appendOrInsertEventSorted(p.events, cloned)
	}

	if cloned.Resource.Kind == "Event" {
		p.applyK8sEvent(cloned)
		return nil
	}
	if cloned.Resource.UID == "" {
		return nil
	}

	p.ensureResourceRecord(cloned)

	uid := cloned.Resource.UID
	if p.retainHistoricalEventArrays {
		p.eventsByResourceUID[uid] = appendOrInsertEventSorted(p.eventsByResourceUID[uid], cloned)
	}

	record := p.resourcesByUID[uid]
	if record == nil {
		return nil
	}

	p.appendResourceVersion(record, cloned)
	p.finishAppliedResourceEvent(uid, cloned, record, cloned.Resource)

	return nil
}

func (p *Projection) applyEventToIndexes(event models.Event) {
	if event.Resource.Kind == "Event" {
		if event.Resource.InvolvedObjectUID == "" {
			return
		}
		involvedUID := event.Resource.InvolvedObjectUID
		if p.retainHistoricalEventArrays {
			p.k8sRawEventsByInvolvedUID[involvedUID] = append(p.k8sRawEventsByInvolvedUID[involvedUID], event)
		}
		p.k8sEventsByInvolvedUID[involvedUID] = append(p.k8sEventsByInvolvedUID[involvedUID], buildK8sEventInfo(event))
		return
	}
	if event.Resource.UID == "" {
		return
	}

	uid := event.Resource.UID
	p.ensureResourceRecord(event)

	record := p.resourcesByUID[uid]
	if record == nil {
		return
	}

	p.appendResourceVersion(record, event)
	p.finishAppliedResourceEvent(uid, event, record, event.Resource)
	if p.retainHistoricalEventArrays {
		p.eventsByResourceUID[uid] = append(p.eventsByResourceUID[uid], event)
	}
}

func (p *Projection) applyK8sEvent(event models.Event) {
	if event.Resource.InvolvedObjectUID == "" {
		return
	}
	involvedUID := event.Resource.InvolvedObjectUID
	if p.retainHistoricalEventArrays {
		p.k8sRawEventsByInvolvedUID[involvedUID] = insertEventSorted(p.k8sRawEventsByInvolvedUID[involvedUID], event)
	}
	p.k8sEventsByInvolvedUID[involvedUID] = insertK8sEventInfoDescending(
		p.k8sEventsByInvolvedUID[involvedUID],
		buildK8sEventInfo(event),
	)
}

func (p *Projection) appendResourceVersion(record *resourceRecord, event models.Event) {
	if p == nil || record == nil {
		return
	}

	var previousData []byte
	if len(record.versions) > 0 {
		previousData = record.versions[len(record.versions)-1].data
	}

	version := resourceVersion{
		eventID:   event.ID,
		timestamp: event.Timestamp,
		eventType: event.Type,
		data:      cloneBytes(event.Data),
	}
	version.identity = buildResourceIdentity(event, parseObject(event.Data), record.versions, previousData)
	version.changeEvent = buildChangeEventInfo(event, version.data, previousData)
	record.versions = append(record.versions, version)
}

func canAppendResourceVersion(record *resourceRecord, event models.Event) bool {
	if record == nil || len(record.versions) == 0 {
		return true
	}

	last := record.versions[len(record.versions)-1]
	if last.timestamp != event.Timestamp {
		return last.timestamp < event.Timestamp
	}
	return last.eventID <= event.ID
}

func (p *Projection) finishAppliedResourceEvent(
	uid string,
	event models.Event,
	record *resourceRecord,
	latestMeta models.ResourceMetadata,
) {
	if p == nil || record == nil {
		return
	}

	p.updateActiveResourceIndex(record)
	p.appendRecentResourceChange(uid, event.Timestamp)
	p.resourceMetaByUID[uid] = latestMeta
	p.updateTimestampBounds(event.Timestamp)
	p.pruneRecentResourceChangesLocked()
}

func (p *Projection) ensureResourceRecord(event models.Event) {
	uid := event.Resource.UID
	if _, ok := p.resourcesByUID[uid]; ok {
		return
	}

	record := &resourceRecord{uid: uid}
	p.resourcesByUID[uid] = record
	key := resourceKey{
		namespace: event.Resource.Namespace,
		kind:      event.Resource.Kind,
		name:      event.Resource.Name,
	}
	p.resourcesByKey[key] = append(p.resourcesByKey[key], record)
	p.orderedResources = insertOrderedResourceKey(p.orderedResources, orderedResourceKey{
		kind:      event.Resource.Kind,
		namespace: event.Resource.Namespace,
		name:      event.Resource.Name,
		uid:       uid,
	})
}

func (p *Projection) updateTimestampBounds(timestamp int64) {
	if p.minTimestampNs < 0 || timestamp < p.minTimestampNs {
		p.minTimestampNs = timestamp
	}
	if p.maxTimestampNs < 0 || timestamp > p.maxTimestampNs {
		p.maxTimestampNs = timestamp
	}
}

func latestResourceMeta(events []models.Event) models.ResourceMetadata {
	if len(events) == 0 {
		return models.ResourceMetadata{}
	}
	return events[len(events)-1].Resource
}

func (p *Projection) updateActiveResourceIndex(record *resourceRecord) {
	if p == nil || record == nil {
		return
	}

	latest := record.latestVersion()
	existing, hasExisting := p.activeResourceKeyByUID[record.uid]
	if latest == nil || latest.eventType == models.EventTypeDelete {
		if hasExisting {
			p.activeOrderedResources = removeOrderedResourceKey(p.activeOrderedResources, existing)
			delete(p.activeResourceKeyByUID, record.uid)
		}
		return
	}

	key := orderedResourceKey{
		kind:      latest.identity.Kind,
		namespace: latest.identity.Namespace,
		name:      latest.identity.Name,
		uid:       latest.identity.UID,
	}
	if hasExisting && existing == key {
		return
	}
	if hasExisting {
		p.activeOrderedResources = removeOrderedResourceKey(p.activeOrderedResources, existing)
	}
	p.activeOrderedResources = insertOrderedResourceKey(p.activeOrderedResources, key)
	p.activeResourceKeyByUID[record.uid] = key
}

func (p *Projection) appendRecentResourceChange(uid string, timestamp int64) {
	if p == nil || uid == "" || timestamp <= 0 {
		return
	}
	p.recentResourceChanges = append(p.recentResourceChanges, recentResourceChange{
		timestamp: timestamp,
		uid:       uid,
	})
}

func (p *Projection) pruneRecentResourceChanges() {
	p.pruneRecentResourceChangesLocked()
}

func (p *Projection) pruneRecentResourceChangesLocked() {
	if p == nil || len(p.recentResourceChanges) == 0 || p.maxTimestampNs <= 0 {
		return
	}

	if p.retentionWindowNs <= 0 {
		return
	}

	cutoff := time.Now().UTC().Add(-time.Duration(p.retentionWindowNs)).UnixNano()
	if cutoff <= 0 {
		return
	}

	pruneIdx := sort.Search(len(p.recentResourceChanges), func(i int) bool {
		return p.recentResourceChanges[i].timestamp >= cutoff
	})
	if pruneIdx <= 0 {
		return
	}
	if pruneIdx >= len(p.recentResourceChanges) {
		p.recentResourceChanges = nil
		return
	}
	p.recentResourceChanges = append([]recentResourceChange(nil), p.recentResourceChanges[pruneIdx:]...)
}

func (p *Projection) resourceEventsForUID(uid string) []models.Event {
	record := p.resourcesByUID[uid]
	if record == nil || len(record.versions) == 0 {
		return nil
	}

	events := make([]models.Event, 0, len(record.versions))
	for i := range record.versions {
		events = append(events, resourceVersionEvent(record.versions[i]))
	}
	return events
}

func (p *Projection) rebuildRecordFromEvents(uid string, events []models.Event) {
	record := p.resourcesByUID[uid]
	if record == nil {
		return
	}

	versions := make([]resourceVersion, 0, len(events))
	for i := range events {
		event := events[i]
		object := parseObject(event.Data)

		var previousData []byte
		if len(versions) > 0 {
			previousData = versions[len(versions)-1].data
		}

		version := resourceVersion{
			eventID:   event.ID,
			timestamp: event.Timestamp,
			eventType: event.Type,
			data:      cloneBytes(event.Data),
		}
		version.identity = buildResourceIdentity(event, object, versions, previousData)
		version.changeEvent = buildChangeEventInfo(event, version.data, previousData)
		versions = append(versions, version)
	}

	record.versions = versions
}

func resourceVersionEvent(version resourceVersion) models.Event {
	return models.Event{
		ID:        version.eventID,
		Timestamp: version.timestamp,
		Type:      version.eventType,
		Resource: models.ResourceMetadata{
			Group:     version.identity.APIGroup,
			Version:   version.identity.Version,
			Kind:      version.identity.Kind,
			Namespace: version.identity.Namespace,
			Name:      version.identity.Name,
			UID:       version.identity.UID,
		},
		Data: cloneBytes(version.data),
	}
}

func insertK8sEventInfoDescending(events []analysisstore.K8sEventInfo, event analysisstore.K8sEventInfo) []analysisstore.K8sEventInfo {
	idx := sort.Search(len(events), func(i int) bool {
		if events[i].Timestamp.Equal(event.Timestamp) {
			return events[i].EventID <= event.EventID
		}
		return events[i].Timestamp.Before(event.Timestamp)
	})

	events = append(events, analysisstore.K8sEventInfo{})
	copy(events[idx+1:], events[idx:])
	events[idx] = event
	return events
}
