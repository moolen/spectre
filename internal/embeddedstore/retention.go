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

	// Trim in place: most records only hold versions inside the window and are left
	// untouched, so a pass costs O(records) rather than O(versions).
	removed := make(map[string]struct{})
	minTimestampNs := int64(-1)
	maxTimestampNs := int64(-1)
	for uid, record := range p.resourcesByUID {
		if record == nil || len(record.versions) == 0 {
			removed[uid] = struct{}{}
			continue
		}
		if record.versions[0].timestamp < cutoffTimestamp {
			retained := retainResourceRecordVersions(record, cutoffTimestamp)
			if len(retained) == 0 {
				removed[uid] = struct{}{}
				continue
			}
			record.versions = retained
		}
		first, last := record.versions[0].timestamp, record.versions[len(record.versions)-1].timestamp
		if minTimestampNs < 0 || first < minTimestampNs {
			minTimestampNs = first
		}
		if maxTimestampNs < 0 || last > maxTimestampNs {
			maxTimestampNs = last
		}
	}

	if len(removed) > 0 {
		for uid := range removed {
			record := p.resourcesByUID[uid]
			delete(p.resourcesByUID, uid)
			delete(p.resourceMetaByUID, uid)
			if record == nil {
				continue
			}
			for key, records := range p.resourceKeysForRecord(record) {
				kept := records[:0]
				for _, candidate := range records {
					if candidate != record {
						kept = append(kept, candidate)
					}
				}
				if len(kept) == 0 {
					delete(p.resourcesByKey, key)
				} else {
					p.resourcesByKey[key] = kept
				}
			}
		}
		var gone []orderedResourceKey
		p.orderedResources.Ascend(func(key orderedResourceKey) bool {
			if _, ok := removed[key.uid]; ok {
				gone = append(gone, key)
			}
			return true
		})
		for _, key := range gone {
			p.orderedResources.Delete(key)
		}
	}

	trimK8sEventInfosBefore(p.k8sEventsByInvolvedUID, cutoffTimestamp)
	for uid, truncated := range p.k8sEventsTruncatedThrough {
		if truncated < cutoffTimestamp {
			delete(p.k8sEventsTruncatedThrough, uid)
		}
	}
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
}

// resourceKeysForRecord returns the resourcesByKey entries that may hold record.
func (p *Projection) resourceKeysForRecord(record *resourceRecord) map[resourceKey][]*resourceRecord {
	keys := make(map[resourceKey][]*resourceRecord, 1)
	seen := make(map[*versionMeta]struct{}, 1)
	for i := range record.versions {
		meta := record.versions[i].meta
		if meta == nil {
			continue
		}
		if _, ok := seen[meta]; ok {
			continue
		}
		seen[meta] = struct{}{}
		key := resourceKey{namespace: meta.Namespace, kind: meta.Kind, name: meta.Name}
		if records, ok := p.resourcesByKey[key]; ok {
			keys[key] = records
		}
	}
	return keys
}

// trimK8sEventInfosBefore drops event infos older than cutoff. Lists are kept newest
// first, so only their tails are cut.
func trimK8sEventInfosBefore(eventsByUID map[string][]analysisstore.K8sEventInfo, cutoffTimestamp int64) {
	for uid, events := range eventsByUID {
		keep := len(events)
		for keep > 0 && events[keep-1].Timestamp.UnixNano() < cutoffTimestamp {
			keep--
		}
		switch {
		case keep == 0:
			delete(eventsByUID, uid)
		case keep < len(events):
			eventsByUID[uid] = append([]analysisstore.K8sEventInfo(nil), events[:keep]...)
		}
	}
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
		if latest.eventType() == models.EventTypeDelete {
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
	// Versions are immutable apart from payload references, so the clone can share
	// meta and labels.
	cloned := version
	cloned.data = cloneBytes(version.data)
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
