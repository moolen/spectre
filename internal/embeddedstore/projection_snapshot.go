package embeddedstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

func (p *Projection) SnapshotEvents() []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.snapshotEventsLocked()
}

func (p *Projection) ExportSnapshot() ProjectionSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	resources := make([]ProjectionResourceSnapshot, 0, len(p.orderedResources))
	for i := range p.orderedResources {
		record := p.resourcesByUID[p.orderedResources[i].uid]
		if record == nil {
			continue
		}
		resources = append(resources, snapshotResourceRecord(record))
	}

	return ProjectionSnapshot{
		Resources:              resources,
		K8sEventsByInvolvedUID: cloneK8sEventsByUID(p.k8sEventsByInvolvedUID),
		MinTimestampNs:         p.minTimestampNs,
		MaxTimestampNs:         p.maxTimestampNs,
	}
}

func (p *Projection) StreamCheckpointResources(emit func(ProjectionResourceSnapshot) error) error {
	if emit == nil {
		return fmt.Errorf("stream checkpoint resources: emit func is nil")
	}

	p.mu.RLock()
	orderedUIDs := make([]string, 0, len(p.orderedResources))
	for i := range p.orderedResources {
		orderedUIDs = append(orderedUIDs, p.orderedResources[i].uid)
	}
	p.mu.RUnlock()

	for i := range orderedUIDs {
		uid := orderedUIDs[i]
		p.mu.RLock()
		record := p.resourcesByUID[uid]
		if record == nil {
			p.mu.RUnlock()
			continue
		}
		snapshot := snapshotResourceRecord(record)
		p.mu.RUnlock()
		if err := emit(snapshot); err != nil {
			return fmt.Errorf("stream checkpoint resources: emit %q: %w", uid, err)
		}
	}
	return nil
}

func (p *Projection) CheckpointState(highWaterMark uint64) checkpointState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return checkpointState{
		FormatVersion:  checkpointFormatVersion,
		HighWaterMark:  highWaterMark,
		MinTimestampNs: p.minTimestampNs,
		MaxTimestampNs: p.maxTimestampNs,
	}
}

func (p *Projection) CheckpointK8sEvents() map[string][]analysisstore.K8sEventInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneK8sEventsByUID(p.k8sEventsByInvolvedUID)
}

func ProjectionFromSnapshot(snapshot ProjectionSnapshot) (*Projection, error) {
	if len(snapshot.Resources) > 0 || len(snapshot.K8sEventsByInvolvedUID) > 0 {
		return projectionFromCompactSnapshot(snapshot), nil
	}

	return BuildProjection(snapshot.Events)
}

func ProjectionFromCheckpointStream(state checkpointState, resources io.Reader, k8s io.Reader) (*Projection, error) {
	if resources == nil {
		return nil, fmt.Errorf("projection checkpoint stream: resources reader is nil")
	}
	if k8s == nil {
		return nil, fmt.Errorf("projection checkpoint stream: k8s reader is nil")
	}

	k8sEvents, err := decodeCheckpointK8sEvents(k8s)
	if err != nil {
		return nil, err
	}

	projection := NewProjection()
	projection.minTimestampNs = state.MinTimestampNs
	projection.maxTimestampNs = state.MaxTimestampNs
	projection.k8sEventsByInvolvedUID = k8sEvents

	decoder := json.NewDecoder(resources)
	for {
		var snapshot ProjectionResourceSnapshot
		if err := decoder.Decode(&snapshot); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("projection checkpoint stream: decode resources stream: %w", err)
		}

		record := restoreResourceRecord(snapshot)
		if record == nil || len(record.versions) == 0 {
			continue
		}
		restoreProjectionResourceRecord(projection, record)
	}

	return projection, nil
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

func (p *Projection) snapshotEventsLocked() []models.Event {
	if len(p.events) > 0 {
		return cloneEvents(p.events)
	}

	events := make([]models.Event, 0)
	for i := range p.orderedResources {
		events = append(events, p.resourceEventsForUID(p.orderedResources[i].uid)...)
	}
	sortEventsIfNeeded(events)
	return events
}

func projectionFromCompactSnapshot(snapshot ProjectionSnapshot) *Projection {
	projection := NewProjection()
	projection.minTimestampNs = snapshot.MinTimestampNs
	projection.maxTimestampNs = snapshot.MaxTimestampNs
	projection.k8sEventsByInvolvedUID = cloneK8sEventsByUID(snapshot.K8sEventsByInvolvedUID)

	for i := range snapshot.Resources {
		record := restoreResourceRecord(snapshot.Resources[i])
		if record == nil || len(record.versions) == 0 {
			continue
		}
		restoreProjectionResourceRecord(projection, record)
	}

	return projection
}

func decodeCheckpointK8sEvents(reader io.Reader) (map[string][]analysisstore.K8sEventInfo, error) {
	decoder := json.NewDecoder(reader)
	var eventsByUID map[string][]analysisstore.K8sEventInfo
	if err := decoder.Decode(&eventsByUID); err != nil {
		if errors.Is(err, io.EOF) {
			return make(map[string][]analysisstore.K8sEventInfo), nil
		}
		return nil, fmt.Errorf("projection checkpoint stream: decode k8s events: %w", err)
	}
	if eventsByUID == nil {
		return make(map[string][]analysisstore.K8sEventInfo), nil
	}
	return cloneK8sEventsByUID(eventsByUID), nil
}

func restoreProjectionResourceRecord(projection *Projection, record *resourceRecord) {
	latest := record.versions[len(record.versions)-1]
	meta := resourceMetadataFromIdentity(latest.identity)
	projection.resourcesByUID[record.uid] = record
	projection.resourceMetaByUID[record.uid] = meta
	key := resourceKey{
		namespace: meta.Namespace,
		kind:      meta.Kind,
		name:      meta.Name,
	}
	projection.resourcesByKey[key] = append(projection.resourcesByKey[key], record)
	projection.orderedResources = insertOrderedResourceKey(projection.orderedResources, orderedResourceKey{
		kind:      meta.Kind,
		namespace: meta.Namespace,
		name:      meta.Name,
		uid:       meta.UID,
	})
}

func snapshotResourceRecord(record *resourceRecord) ProjectionResourceSnapshot {
	if record == nil {
		return ProjectionResourceSnapshot{}
	}

	versions := make([]ProjectionResourceVersionSnapshot, 0, len(record.versions))
	for i := range record.versions {
		version := record.versions[i]
		versions = append(versions, ProjectionResourceVersionSnapshot{
			EventID:     version.eventID,
			Timestamp:   version.timestamp,
			EventType:   version.eventType,
			Identity:    copyIdentity(&version),
			Data:        cloneBytes(version.data),
			ChangeEvent: cloneChangeEventInfo(version.changeEvent),
		})
	}

	return ProjectionResourceSnapshot{
		UID:      record.uid,
		Versions: versions,
	}
}

func restoreResourceRecord(snapshot ProjectionResourceSnapshot) *resourceRecord {
	if len(snapshot.Versions) == 0 {
		return nil
	}

	record := &resourceRecord{
		uid:      snapshot.UID,
		versions: make([]resourceVersion, 0, len(snapshot.Versions)),
	}
	for i := range snapshot.Versions {
		version := snapshot.Versions[i]
		record.versions = append(record.versions, resourceVersion{
			eventID:     version.EventID,
			timestamp:   version.Timestamp,
			eventType:   version.EventType,
			identity:    cloneGraphIdentity(version.Identity),
			data:        cloneBytes(version.Data),
			changeEvent: cloneChangeEventInfo(version.ChangeEvent),
		})
	}

	return record
}

func resourceMetadataFromIdentity(identity graph.ResourceIdentity) models.ResourceMetadata {
	return models.ResourceMetadata{
		Group:     identity.APIGroup,
		Version:   identity.Version,
		Kind:      identity.Kind,
		Namespace: identity.Namespace,
		Name:      identity.Name,
		UID:       identity.UID,
	}
}

func cloneK8sEventsByUID(input map[string][]analysisstore.K8sEventInfo) map[string][]analysisstore.K8sEventInfo {
	if len(input) == 0 {
		return make(map[string][]analysisstore.K8sEventInfo)
	}

	cloned := make(map[string][]analysisstore.K8sEventInfo, len(input))
	for uid, events := range input {
		items := make([]analysisstore.K8sEventInfo, len(events))
		for i := range events {
			items[i] = cloneK8sEventInfo(events[i])
		}
		cloned[uid] = items
	}

	return cloned
}

func cloneChangeEventInfo(input analysisstore.ChangeEventInfo) analysisstore.ChangeEventInfo {
	cloned := input
	cloned.Data = cloneBytes(input.Data)
	return cloned
}

func cloneK8sEventInfo(input analysisstore.K8sEventInfo) analysisstore.K8sEventInfo {
	return input
}

func cloneGraphIdentity(input graph.ResourceIdentity) graph.ResourceIdentity {
	cloned := input
	cloned.Labels = copyStringMap(input.Labels)
	return cloned
}

func sortEventsIfNeeded(events []models.Event) {
	if len(events) <= 1 {
		return
	}

	sort.SliceStable(events, func(i, j int) bool {
		return compareEventOrder(events[i], events[j]) < 0
	})
}
