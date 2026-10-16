package embeddedstore

import (
	"sort"

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
		p.k8sEventsByInvolvedUID[involvedUID] = p.capK8sEventInfos(involvedUID, p.k8sEventsByInvolvedUID[involvedUID])
	}

	if !p.retainHistoricalEventArrays {
		p.events = nil
		clear(p.eventsByResourceUID)
		clear(p.k8sRawEventsByInvolvedUID)
	}
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
	if !p.retainHistoricalEventArrays {
		insertResourceVersion(record, cloned)
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
	p.k8sEventsByInvolvedUID[involvedUID] = p.capK8sEventInfos(involvedUID, insertK8sEventInfoDescending(
		p.k8sEventsByInvolvedUID[involvedUID],
		buildK8sEventInfo(event),
	))
}

func (p *Projection) appendResourceVersion(record *resourceRecord, event models.Event) {
	if p == nil || record == nil {
		return
	}

	version, state := buildResourceVersion(event, record.versions, record.latestChangeState())
	record.versions = append(record.versions, version)
	record.latest = state
}

func canAppendResourceVersion(record *resourceRecord, event models.Event) bool {
	if record == nil || len(record.versions) == 0 {
		return true
	}

	last := record.versions[len(record.versions)-1]
	if last.timestamp != event.Timestamp {
		return last.timestamp < event.Timestamp
	}
	return last.eventID() <= event.ID
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

	p.updateTimestampBounds(event.Timestamp)
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
	p.orderedResources.Insert(orderedResourceKey{
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
	state := recordChangeState{known: true}
	for i := range events {
		var version resourceVersion
		version, state = buildResourceVersion(events[i], versions, state)
		versions = append(versions, version)
	}

	record.versions = versions
	record.latest = state
}

func resourceVersionEvent(version resourceVersion) models.Event {
	return models.Event{
		ID:        version.eventID(),
		Timestamp: version.timestamp,
		Type:      version.eventType(),
		Resource: models.ResourceMetadata{
			Group:     version.meta.APIGroup,
			Version:   version.meta.Version,
			Kind:      version.meta.Kind,
			Namespace: version.meta.Namespace,
			Name:      version.meta.Name,
			UID:       version.meta.UID,
		},
		Data: versionPayloadCopy(&version),
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

// maxK8sEventInfosPerUID bounds the event summaries kept in memory per involved object.
// Older summaries stay in segments and are found through the associated-UID index.
const maxK8sEventInfosPerUID = 64

// capK8sEventInfos trims a newest-first list to the cap and records what was dropped.
func (p *Projection) capK8sEventInfos(involvedUID string, events []analysisstore.K8sEventInfo) []analysisstore.K8sEventInfo {
	if len(events) <= maxK8sEventInfosPerUID {
		return events
	}
	newestDropped := events[maxK8sEventInfosPerUID].Timestamp.UnixNano()
	if newestDropped > p.k8sEventsTruncatedThrough[involvedUID] {
		p.k8sEventsTruncatedThrough[involvedUID] = newestDropped
	}
	clear(events[maxK8sEventInfosPerUID:])
	return events[:maxK8sEventInfosPerUID]
}

// k8sEventInfosComplete reports whether the in-memory infos for uid cover every event
// at or after startTimeNs.
func (p *Projection) k8sEventInfosComplete(uid string, startTimeNs int64) bool {
	truncated, ok := p.k8sEventsTruncatedThrough[uid]
	return !ok || startTimeNs > truncated
}

// insertResourceVersion places an out-of-order event into a record's history. Only the
// inserted version and the one after it are reclassified, and the other versions keep
// their payload references.
func insertResourceVersion(record *resourceRecord, event models.Event) {
	versions := record.versions
	idx := sort.Search(len(versions), func(i int) bool {
		if versions[i].timestamp != event.Timestamp {
			return versions[i].timestamp > event.Timestamp
		}
		return versions[i].eventID() > event.ID
	})

	previous := recordChangeState{known: true}
	if idx > 0 {
		data := versionPayload(&versions[idx-1])
		previous = newRecordChangeState(data, parseObject(data))
	}
	version, state := buildResourceVersion(event, versions[:idx], previous)

	versions = append(versions, resourceVersion{})
	copy(versions[idx+1:], versions[idx:])
	versions[idx] = version

	if idx+1 < len(versions) {
		next := &versions[idx+1]
		data := versionPayload(next)
		configChanged, statusChanged := changeKindsFromStates(state, newRecordChangeState(data, parseObject(data)))
		next.flags = 0
		if configChanged {
			next.flags |= versionFlagConfigChanged
		}
		if statusChanged {
			next.flags |= versionFlagStatusChanged
		}
	}
	if idx == 0 && len(versions) > 1 {
		// The resource is now first seen at the inserted event.
		replaced := make(map[*versionMeta]*versionMeta)
		for i := 1; i < len(versions); i++ {
			meta := versions[i].meta
			if meta == nil {
				continue
			}
			updated, ok := replaced[meta]
			if !ok {
				copied := *meta
				copied.FirstSeen = event.Timestamp
				updated = &copied
				replaced[meta] = updated
			}
			versions[i].meta = updated
		}
	}
	record.versions = versions
	if idx == len(versions)-1 {
		record.latest = state
	}
}
