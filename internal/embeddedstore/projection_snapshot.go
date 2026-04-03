package embeddedstore

import "github.com/moolen/spectre/internal/models"

func (p *Projection) SnapshotEvents() []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return cloneEvents(p.events)
}

func (p *Projection) ExportSnapshot() ProjectionSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProjectionSnapshot{
		Events: cloneEvents(p.events),
	}
}

func ProjectionFromSnapshot(snapshot ProjectionSnapshot) (*Projection, error) {
	return BuildProjection(snapshot.Events)
}

func (p *Projection) ImportSnapshot(snapshot ProjectionSnapshot) error {
	rebuilt, err := ProjectionFromSnapshot(snapshot)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.replaceStateLocked(rebuilt)
	return nil
}

func (p *Projection) replaceStateLocked(other *Projection) {
	p.events = other.events
	p.eventsByResourceUID = other.eventsByResourceUID
	p.resourceMetaByUID = other.resourceMetaByUID
	p.resourcesByUID = other.resourcesByUID
	p.resourcesByKey = other.resourcesByKey
	p.k8sRawEventsByInvolvedUID = other.k8sRawEventsByInvolvedUID
	p.k8sEventsByInvolvedUID = other.k8sEventsByInvolvedUID
	p.orderedResources = other.orderedResources
	p.minTimestampNs = other.minTimestampNs
	p.maxTimestampNs = other.maxTimestampNs
}
