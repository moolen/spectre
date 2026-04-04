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

	sort.Slice(projection.orderedResources, func(i, j int) bool {
		return compareOrderedResourceKey(projection.orderedResources[i], projection.orderedResources[j]) < 0
	})

	for uid := range projection.resourcesByUID {
		projection.rebuildRecord(uid)
	}
	for involvedUID := range projection.k8sEventsByInvolvedUID {
		sort.Slice(projection.k8sEventsByInvolvedUID[involvedUID], func(i, j int) bool {
			left := projection.k8sEventsByInvolvedUID[involvedUID][i]
			right := projection.k8sEventsByInvolvedUID[involvedUID][j]
			if left.Timestamp.Equal(right.Timestamp) {
				return left.EventID > right.EventID
			}
			return left.Timestamp.After(right.Timestamp)
		})
	}

	return projection, nil
}

func (p *Projection) appendReplayEvent(event models.Event) {
	cloned := cloneEvent(event)
	p.events = append(p.events, cloned)
	p.applyEventToIndexes(cloned)
}

func (p *Projection) finalizeReplayBuild() {
	sort.Slice(p.orderedResources, func(i, j int) bool {
		return compareOrderedResourceKey(p.orderedResources[i], p.orderedResources[j]) < 0
	})

	for uid := range p.resourcesByUID {
		p.rebuildRecord(uid)
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
}

func (p *Projection) Apply(event models.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cloned := cloneEvent(event)
	p.events = insertEventSorted(p.events, cloned)

	if cloned.Resource.Kind == "Event" {
		p.applyK8sEvent(cloned)
		return nil
	}
	if cloned.Resource.UID == "" {
		return nil
	}

	p.ensureResourceRecord(cloned)

	uid := cloned.Resource.UID
	p.eventsByResourceUID[uid] = insertEventSorted(p.eventsByResourceUID[uid], cloned)
	p.resourceMetaByUID[uid] = latestResourceMeta(p.eventsByResourceUID[uid])
	p.rebuildRecord(uid)
	p.updateTimestampBounds(cloned.Timestamp)

	return nil
}

func (p *Projection) applyEventToIndexes(event models.Event) {
	if event.Resource.Kind == "Event" {
		if event.Resource.InvolvedObjectUID == "" {
			return
		}
		involvedUID := event.Resource.InvolvedObjectUID
		p.k8sRawEventsByInvolvedUID[involvedUID] = append(p.k8sRawEventsByInvolvedUID[involvedUID], event)
		p.k8sEventsByInvolvedUID[involvedUID] = append(p.k8sEventsByInvolvedUID[involvedUID], buildK8sEventInfo(event))
		return
	}
	if event.Resource.UID == "" {
		return
	}

	uid := event.Resource.UID
	p.ensureResourceRecord(event)
	p.eventsByResourceUID[uid] = append(p.eventsByResourceUID[uid], event)
	p.resourceMetaByUID[uid] = event.Resource
	p.updateTimestampBounds(event.Timestamp)
}

func (p *Projection) applyK8sEvent(event models.Event) {
	if event.Resource.InvolvedObjectUID == "" {
		return
	}
	involvedUID := event.Resource.InvolvedObjectUID
	p.k8sRawEventsByInvolvedUID[involvedUID] = insertEventSorted(p.k8sRawEventsByInvolvedUID[involvedUID], event)
	p.k8sEventsByInvolvedUID[involvedUID] = insertK8sEventInfoDescending(
		p.k8sEventsByInvolvedUID[involvedUID],
		buildK8sEventInfo(event),
	)
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
