package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
)

func embeddedRetentionWindow(days int) time.Duration {
	if days <= 0 {
		return 0
	}
	return time.Duration(days) * 24 * time.Hour
}

func embeddedRetentionWindowNs(days int) int64 {
	window := embeddedRetentionWindow(days)
	if window <= 0 {
		return 0
	}
	return window.Nanoseconds()
}

func embeddedRetentionCutoffTimestamp(days int, now time.Time) int64 {
	window := embeddedRetentionWindow(days)
	if window <= 0 {
		return 0
	}
	return now.UTC().Add(-window).UnixNano()
}

func filterEventsByTimestamp(events []models.Event, cutoffTimestamp int64) []models.Event {
	if len(events) == 0 {
		return nil
	}
	if cutoffTimestamp <= 0 {
		return cloneEvents(events)
	}

	filtered := make([]models.Event, 0, len(events))
	for i := range events {
		if events[i].Timestamp < cutoffTimestamp {
			continue
		}
		filtered = append(filtered, cloneEvent(events[i]))
	}
	return filtered
}

func filterK8sEventInfosByTimestamp(
	input map[string][]analysisstore.K8sEventInfo,
	cutoffTimestamp int64,
) map[string][]analysisstore.K8sEventInfo {
	if len(input) == 0 {
		return make(map[string][]analysisstore.K8sEventInfo)
	}
	if cutoffTimestamp <= 0 {
		return cloneK8sEventsByUID(input)
	}

	filtered := make(map[string][]analysisstore.K8sEventInfo, len(input))
	for uid, events := range input {
		items := make([]analysisstore.K8sEventInfo, 0, len(events))
		for i := range events {
			if events[i].Timestamp.UnixNano() < cutoffTimestamp {
				continue
			}
			items = append(items, cloneK8sEventInfo(events[i]))
		}
		if len(items) > 0 {
			filtered[uid] = items
		}
	}
	return filtered
}

func (p *Projection) SetRetentionWindowDays(days int) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.retentionWindowNs = embeddedRetentionWindowNs(days)
	p.pruneRecentResourceChangesLocked()
}

func (p *Projection) retentionWindowLocked() int64 {
	if p == nil {
		return 0
	}
	return p.retentionWindowNs
}

func (p *Projection) RetentionWindowNs() int64 {
	if p == nil {
		return 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.retentionWindowNs
}

func (p *Projection) ApplyRetentionCutoff(cutoffTimestamp int64) {
	if p == nil || cutoffTimestamp <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.applyRetentionCutoffLocked(cutoffTimestamp)
}

func (p *Projection) applyRetentionCutoffLocked(cutoffTimestamp int64) {
	if p == nil || cutoffTimestamp <= 0 {
		return
	}

	newResourcesByUID := make(map[string]*resourceRecord, len(p.resourcesByUID))
	newResourceMetaByUID := make(map[string]models.ResourceMetadata, len(p.resourceMetaByUID))
	newResourcesByKey := make(map[resourceKey][]*resourceRecord, len(p.resourcesByKey))
	newOrderedResources := make([]orderedResourceKey, 0, len(p.orderedResources))
	newActiveOrderedResources := make([]orderedResourceKey, 0, len(p.activeOrderedResources))
	newActiveResourceKeyByUID := make(map[string]orderedResourceKey, len(p.activeResourceKeyByUID))
	newRecentResourceChanges := make([]recentResourceChange, 0, len(p.recentResourceChanges))

	minTimestampNs := int64(-1)
	maxTimestampNs := int64(-1)

	for i := range p.orderedResources {
		uid := p.orderedResources[i].uid
		record := p.resourcesByUID[uid]
		retained := retainResourceRecordVersions(record, cutoffTimestamp)
		if len(retained) == 0 {
			continue
		}

		clonedRecord := &resourceRecord{
			uid:      uid,
			versions: retained,
		}
		newResourcesByUID[uid] = clonedRecord

		latest := clonedRecord.latestVersion()
		if latest == nil {
			continue
		}
		meta := resourceMetadataFromIdentity(latest.identity)
		newResourceMetaByUID[uid] = meta
		key := resourceKey{
			namespace: meta.Namespace,
			kind:      meta.Kind,
			name:      meta.Name,
		}
		newResourcesByKey[key] = append(newResourcesByKey[key], clonedRecord)
		newOrderedResources = append(newOrderedResources, orderedResourceKey{
			kind:      meta.Kind,
			namespace: meta.Namespace,
			name:      meta.Name,
			uid:       meta.UID,
		})
		if latest.eventType != models.EventTypeDelete {
			activeKey := orderedResourceKey{
				kind:      meta.Kind,
				namespace: meta.Namespace,
				name:      meta.Name,
				uid:       meta.UID,
			}
			newActiveOrderedResources = append(newActiveOrderedResources, activeKey)
			newActiveResourceKeyByUID[uid] = activeKey
		}

		for j := range clonedRecord.versions {
			version := clonedRecord.versions[j]
			if minTimestampNs < 0 || version.timestamp < minTimestampNs {
				minTimestampNs = version.timestamp
			}
			if maxTimestampNs < 0 || version.timestamp > maxTimestampNs {
				maxTimestampNs = version.timestamp
			}
			if version.timestamp >= cutoffTimestamp {
				newRecentResourceChanges = append(newRecentResourceChanges, recentResourceChange{
					timestamp: version.timestamp,
					uid:       uid,
				})
			}
		}
	}

	sort.Slice(newActiveOrderedResources, func(i, j int) bool {
		return compareOrderedResourceKey(newActiveOrderedResources[i], newActiveOrderedResources[j]) < 0
	})

	p.resourcesByUID = newResourcesByUID
	p.resourceMetaByUID = newResourceMetaByUID
	p.resourcesByKey = newResourcesByKey
	p.orderedResources = newOrderedResources
	p.activeOrderedResources = newActiveOrderedResources
	p.activeResourceKeyByUID = newActiveResourceKeyByUID
	p.k8sEventsByInvolvedUID = filterK8sEventInfosByTimestamp(p.k8sEventsByInvolvedUID, cutoffTimestamp)
	p.recentResourceChanges = newRecentResourceChanges
	sortRecentResourceChanges(p.recentResourceChanges)
	p.minTimestampNs = minTimestampNs
	p.maxTimestampNs = maxTimestampNs

	if p.retainHistoricalEventArrays {
		p.events = filterEventsByTimestamp(p.events, cutoffTimestamp)

		filteredByUID := make(map[string][]models.Event, len(p.eventsByResourceUID))
		for uid, events := range p.eventsByResourceUID {
			filtered := filterEventsByTimestamp(events, cutoffTimestamp)
			if len(filtered) > 0 {
				filteredByUID[uid] = filtered
			}
		}
		p.eventsByResourceUID = filteredByUID

		filteredK8sRaw := make(map[string][]models.Event, len(p.k8sRawEventsByInvolvedUID))
		for uid, events := range p.k8sRawEventsByInvolvedUID {
			filtered := filterEventsByTimestamp(events, cutoffTimestamp)
			if len(filtered) > 0 {
				filteredK8sRaw[uid] = filtered
			}
		}
		p.k8sRawEventsByInvolvedUID = filteredK8sRaw
	}

	p.pruneRecentResourceChangesLocked()
}

func retainResourceRecordVersions(record *resourceRecord, cutoffTimestamp int64) []resourceVersion {
	if record == nil || len(record.versions) == 0 {
		return nil
	}
	if cutoffTimestamp <= 0 {
		return cloneResourceVersions(record.versions)
	}

	retainedStart := sort.Search(len(record.versions), func(i int) bool {
		return record.versions[i].timestamp >= cutoffTimestamp
	})

	if retainedStart >= len(record.versions) {
		latest := record.versions[len(record.versions)-1]
		if latest.eventType == models.EventTypeDelete {
			return nil
		}
		return []resourceVersion{cloneResourceVersion(latest)}
	}

	retained := make([]resourceVersion, 0, len(record.versions)-retainedStart+1)
	if retainedStart > 0 {
		sentinel := record.versions[retainedStart-1]
		firstRetained := record.versions[retainedStart]
		if !checkpointEquivalentState(sentinel, firstRetained) {
			retained = append(retained, cloneResourceVersion(sentinel))
		}
	}
	for i := retainedStart; i < len(record.versions); i++ {
		retained = append(retained, cloneResourceVersion(record.versions[i]))
	}
	return retained
}

func cloneResourceVersions(versions []resourceVersion) []resourceVersion {
	if len(versions) == 0 {
		return nil
	}
	cloned := make([]resourceVersion, 0, len(versions))
	for i := range versions {
		cloned = append(cloned, cloneResourceVersion(versions[i]))
	}
	return cloned
}

func cloneResourceVersion(version resourceVersion) resourceVersion {
	cloned := version
	cloned.identity = copyIdentity(&version)
	cloned.data = cloneBytes(version.data)
	cloned.changeEvent = cloneChangeEventInfo(version.changeEvent)
	return cloned
}

func reconcileEmbeddedRetention(
	ctx context.Context,
	rootDir string,
	manifest Manifest,
	cutoffTimestamp int64,
	rewriteActiveTail bool,
) (Manifest, error) {
	if cutoffTimestamp <= 0 {
		return manifest, nil
	}

	updatedManifest := manifest
	var err error

	updatedManifest, err = pruneExpiredCheckpoints(rootDir, updatedManifest, cutoffTimestamp)
	if err != nil {
		return manifest, err
	}

	updatedManifest.ActiveSegments, err = rewriteSegmentsForRetention(ctx, rootDir, updatedManifest.ActiveSegments, cutoffTimestamp)
	if err != nil {
		return manifest, err
	}

	if rewriteActiveTail {
		updatedManifest, err = rewriteTailForRetention(rootDir, updatedManifest, cutoffTimestamp)
		if err != nil {
			return manifest, err
		}
	}

	if err := storeManifest(rootDir, updatedManifest); err != nil {
		return manifest, err
	}
	return updatedManifest, nil
}

func pruneExpiredCheckpoints(rootDir string, manifest Manifest, cutoffTimestamp int64) (Manifest, error) {
	if cutoffTimestamp <= 0 || len(manifest.Checkpoints) == 0 {
		return manifest, nil
	}

	activeCheckpointID := manifest.ActiveCheckpoint.ID
	if activeCheckpointID == "" {
		activeCheckpointID = latestCheckpointMeta(manifest.Checkpoints).ID
	}

	kept := make([]CheckpointMeta, 0, len(manifest.Checkpoints))
	removedAny := false
	for i := range manifest.Checkpoints {
		checkpoint := manifest.Checkpoints[i]
		if checkpoint.ID == "" {
			continue
		}
		if checkpoint.ID == activeCheckpointID {
			kept = append(kept, checkpoint)
			continue
		}

		var state checkpointState
		if err := loadCheckpointState(filepath.Join(rootDir, checkpointsDirName, checkpoint.ID), &state); err != nil {
			return manifest, fmt.Errorf("load checkpoint state %q: %w", checkpoint.ID, err)
		}
		if state.MaxTimestampNs >= cutoffTimestamp {
			kept = append(kept, checkpoint)
			continue
		}
		if err := os.RemoveAll(filepath.Join(rootDir, checkpointsDirName, checkpoint.ID)); err != nil {
			return manifest, fmt.Errorf("remove expired checkpoint %q: %w", checkpoint.ID, err)
		}
		removedAny = true
	}

	sort.Slice(kept, func(i, j int) bool {
		if kept[i].HighWaterMark != kept[j].HighWaterMark {
			return kept[i].HighWaterMark < kept[j].HighWaterMark
		}
		return kept[i].ID < kept[j].ID
	})

	updatedManifest := manifest
	updatedManifest.Checkpoints = kept
	if len(kept) > 0 {
		updatedManifest.ActiveCheckpoint = latestCheckpointMeta(kept)
	} else {
		updatedManifest.ActiveCheckpoint = CheckpointMeta{}
	}
	if removedAny {
		if err := syncPath(filepath.Join(rootDir, checkpointsDirName)); err != nil {
			return manifest, fmt.Errorf("sync checkpoints dir after retention: %w", err)
		}
	}
	return updatedManifest, nil
}

func rewriteSegmentsForRetention(
	ctx context.Context,
	rootDir string,
	segments []SegmentMeta,
	cutoffTimestamp int64,
) ([]SegmentMeta, error) {
	if cutoffTimestamp <= 0 || len(segments) == 0 {
		return segments, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	updatedSegments := make([]SegmentMeta, 0, len(segments))
	segmentsRoot := filepath.Join(rootDir, segmentsDirName)
	removedAny := false

	for i := range segments {
		current := segments[i]
		reader, err := openSegmentReader(rootDir, current.bundleMeta())
		if err != nil {
			return nil, fmt.Errorf("open retention segment %q: %w", current.ID, err)
		}

		meta, err := reader.EnsureBundleMeta()
		if err != nil {
			return nil, fmt.Errorf("load retention segment metadata %q: %w", current.ID, err)
		}
		if meta.EventCount == 0 || meta.MaxTimestamp < cutoffTimestamp {
			if err := os.RemoveAll(filepath.Join(segmentsRoot, current.ID)); err != nil {
				return nil, fmt.Errorf("remove expired segment %q: %w", current.ID, err)
			}
			removedAny = true
			continue
		}
		if meta.MinTimestamp >= cutoffTimestamp {
			updatedSegments = append(updatedSegments, current)
			continue
		}

		events, err := reader.ScanTimeRange(ctx, cutoffTimestamp, meta.MaxTimestamp)
		if err != nil {
			return nil, fmt.Errorf("scan retention segment %q: %w", current.ID, err)
		}
		if len(events) == 0 {
			if err := os.RemoveAll(filepath.Join(segmentsRoot, current.ID)); err != nil {
				return nil, fmt.Errorf("remove expired segment %q: %w", current.ID, err)
			}
			removedAny = true
			continue
		}

		replacementID := newSegmentID(current.HighWaterMark)
		replacementMeta, err := writeSegment(rootDir, replacementID, events)
		if err != nil {
			return nil, fmt.Errorf("rewrite retention segment %q: %w", current.ID, err)
		}
		if err := os.RemoveAll(filepath.Join(segmentsRoot, current.ID)); err != nil {
			return nil, fmt.Errorf("remove rewritten segment %q: %w", current.ID, err)
		}
		removedAny = true
		updatedSegments = append(updatedSegments, segmentMetaFromBundle(replacementMeta, current.HighWaterMark))
	}

	if removedAny {
		if err := syncPath(segmentsRoot); err != nil {
			return nil, fmt.Errorf("sync segments dir after retention: %w", err)
		}
	}
	return updatedSegments, nil
}

func rewriteTailForRetention(rootDir string, manifest Manifest, cutoffTimestamp int64) (Manifest, error) {
	if cutoffTimestamp <= 0 || manifest.ActiveTail.ID == "" {
		return manifest, nil
	}

	tail, err := openTailJournal(rootDir, manifest.ActiveTail)
	if err != nil {
		return manifest, fmt.Errorf("open active tail for retention: %w", err)
	}
	defer func() {
		_ = tail.Close()
	}()

	retained := make([]models.Event, 0, manifest.ActiveTail.EventCount)
	if err := tail.ReplaySince(context.Background(), manifest.ActiveTail.BaseHighWaterMark, func(event models.Event, _ uint64) error {
		if event.Timestamp >= cutoffTimestamp {
			retained = append(retained, cloneEvent(event))
		}
		return nil
	}); err != nil {
		return manifest, fmt.Errorf("replay active tail for retention: %w", err)
	}

	baseHighWaterMark := maxUint64(
		manifest.ActiveTail.BaseHighWaterMark,
		manifest.ActiveCheckpoint.HighWaterMark,
		maxSegmentHighWaterMark(manifest.ActiveSegments),
		manifest.FlushHighWaterMark,
	)
	updatedMeta, err := tail.Reset(baseHighWaterMark)
	if err != nil {
		return manifest, fmt.Errorf("reset active tail for retention: %w", err)
	}
	if len(retained) > 0 {
		updatedMeta, err = tail.AppendBatch(context.Background(), baseHighWaterMark+1, retained)
		if err != nil {
			return manifest, fmt.Errorf("rewrite active tail for retention: %w", err)
		}
	}

	updatedManifest := manifest
	updatedManifest.ActiveTail = updatedMeta
	return updatedManifest, nil
}

func rebuildHotStoreWithRetention(existing *hotStore, metrics *Metrics, cutoffTimestamp int64) *hotStore {
	if existing == nil {
		return nil
	}

	filtered := filterEventsByTimestamp(existing.ExtractFlushBatch(0).Events, cutoffTimestamp)
	rebuilt := newHotStore(existing.config, metrics)
	if len(filtered) > 0 {
		rebuilt.Append(filtered)
	}
	return rebuilt
}
