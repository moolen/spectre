package embeddedstore

import (
	"sort"

	"github.com/moolen/spectre/internal/models"
)

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
