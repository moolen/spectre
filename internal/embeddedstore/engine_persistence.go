package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func (e *Engine) Flush(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("flush embedded engine: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}

	return e.flushLocked(ctx)
}

func (e *Engine) flushLocked(ctx context.Context) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !e.ready.Load() {
		return nil
	}

	batch := e.hot.ExtractFlushBatch(0)
	if len(batch.Events) == 0 {
		return nil
	}

	start := time.Now()
	byteCount := estimatedEventsSizeBytes(batch.Events)
	defer func() {
		e.metrics.RecordFlush(time.Since(start), len(batch.Events), byteCount, err)
	}()

	segmentID := newSegmentID(e.nextHighWaterMark)
	meta, locations, err := writeSegmentLocated(e.rootDir, segmentID, batch.Events)
	if err != nil {
		return fmt.Errorf("flush embedded engine: write segment: %w", err)
	}

	reader, err := openSegmentReader(e.rootDir, meta)
	if err != nil {
		return fmt.Errorf("flush embedded engine: open segment reader: %w", err)
	}

	updatedManifest := e.manifest
	if len(updatedManifest.ActiveSegments) == 0 && len(e.manifest.RetiredSegments) == 0 {
		// Every segment of this store is written in the current format, so there is
		// nothing for the startup index migration to do.
		updatedManifest.SegmentIndexGeneration = associatedIndexGeneration
	}
	updatedManifest.ActiveSegments = append(updatedManifest.ActiveSegments, segmentMetaFromBundle(meta, e.nextHighWaterMark))
	updatedManifest.FlushHighWaterMark = e.nextHighWaterMark
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("flush embedded engine: store manifest: %w", err)
	}

	removed := e.hot.CommitFlushedBatch(batch)
	if removed != len(batch.Events) {
		return fmt.Errorf("flush embedded engine: removed %d of %d flushed events", removed, len(batch.Events))
	}

	e.manifest = updatedManifest
	e.segmentReaders = append(e.segmentReaders, reader)
	e.projection.AttachSegmentLocations(segmentID, locations)
	if reader.MayContainKind("Event") {
		if warmErr := reader.ensureAssociatedIndexLoaded(); warmErr != nil && e.logger != nil {
			e.logger.WarnWithFields(
				"embedded flush warmup failed to load associated index",
				logging.Field("segment_id", reader.ID()),
				logging.Field("error", warmErr.Error()),
			)
		}
	}
	e.refreshQueryPlanner()
	e.metrics.SetActiveSegments(len(e.segmentReaders))
	e.scheduleCompactionLocked()
	return nil
}

func (e *Engine) shouldFlushHotBySizeLocked() bool {
	if e.config.HotMaxEvents > 0 && e.hot.Len() >= e.config.HotMaxEvents {
		return true
	}
	if e.config.SegmentTargetBytes <= 0 {
		return false
	}
	return e.hot.SizeBytes() >= e.config.SegmentTargetBytes
}

func estimatedEventsSizeBytes(events []models.Event) int64 {
	var total int64
	for i := range events {
		total += estimatedEventSizeBytes(&events[i])
	}
	return total
}

// estimatedEventSizeBytes approximates an event's serialized footprint; exact framing
// size is unnecessary for threshold triggering.
func estimatedEventSizeBytes(event *models.Event) int64 {
	total := int64(len(event.ID) + len(event.Type))
	total += int64(len(event.Resource.UID) + len(event.Resource.Namespace) + len(event.Resource.Kind))
	total += int64(len(event.Resource.Name) + len(event.Resource.Group) + len(event.Resource.Version))
	total += int64(len(event.Data))
	return total + 32
}

func (e *Engine) logAutoFlushError(message string, err error) {
	if e == nil || err == nil {
		return
	}
	if e.logger == nil {
		e.logger = logging.GetLogger("embedded.engine")
	}

	e.logger.ErrorWithErr(message, err)
}

// Checkpoint persists the projection so restarts only replay the tail. It runs in three
// phases: a short locked phase flushes, applies retention, captures a snapshot and
// rotates the tail; the snapshot is then encoded without the engine lock while ingest
// continues; a final locked phase publishes the checkpoint and removes what it
// superseded.
func (e *Engine) Checkpoint(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("checkpoint embedded engine: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.checkpointMu.Lock()
	defer e.checkpointMu.Unlock()
	return e.runCheckpoint(ctx)
}

func (e *Engine) runCheckpoint(ctx context.Context) (err error) {
	e.mu.Lock()
	if e.closed || !e.ready.Load() {
		e.mu.Unlock()
		return nil
	}
	start := time.Now()
	defer func() {
		e.metrics.RecordCheckpoint(time.Since(start), err)
	}()
	snapshot, retiredBefore, err := e.beginCheckpointLocked(ctx)
	e.mu.Unlock()
	if err != nil {
		return err
	}

	meta, err := writeCheckpointV3(e.rootDir, e.projection, snapshot)
	if err != nil {
		return fmt.Errorf("checkpoint embedded engine: write checkpoint: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	return e.commitCheckpointLocked(meta, retiredBefore)
}

// beginCheckpointLocked flushes, applies retention, rotates the tail at the checkpoint's
// high-water mark and captures the snapshot to encode.
func (e *Engine) beginCheckpointLocked(ctx context.Context) (*checkpointSnapshot, map[string]struct{}, error) {
	// Flush first so every version at or below the high-water mark is in a segment: the
	// checkpoint then stores references instead of payloads.
	if err := e.flushLocked(ctx); err != nil {
		return nil, nil, fmt.Errorf("checkpoint embedded engine: flush: %w", err)
	}
	// Apply retention before snapshotting so the checkpoint reflects the pruned state and
	// the segments retention replaced can be removed once it is written.
	retentionCutoffTimestamp := embeddedRetentionCutoffTimestamp(e.config.EmbeddedRetentionDays, time.Now())
	if err := e.applyRetentionLocked(ctx, retentionCutoffTimestamp); err != nil {
		return nil, nil, fmt.Errorf("checkpoint embedded engine: apply retention: %w", err)
	}

	highWaterMark := e.nextHighWaterMark
	snapshot := e.projection.captureCheckpointSnapshot(highWaterMark)
	retiredBefore := make(map[string]struct{}, len(e.manifest.RetiredSegments))
	for _, retired := range e.manifest.RetiredSegments {
		retiredBefore[retired.ID] = struct{}{}
	}

	// Start a new tail at the high-water mark. The previous tail stays on disk (listed in
	// the manifest) until the checkpoint is published, so a crash in between can still
	// replay it on top of the older checkpoint.
	if e.tail != nil && e.tail.meta.LastHighWaterMark > e.tail.meta.BaseHighWaterMark {
		previous := e.tail.meta
		next, err := e.tail.Rotate(highWaterMark)
		if err != nil {
			return nil, nil, fmt.Errorf("checkpoint embedded engine: rotate tail journal: %w", err)
		}
		updatedManifest := e.manifest
		updatedManifest.PreviousTails = append(append([]TailJournalMeta(nil), updatedManifest.PreviousTails...), previous)
		updatedManifest.ActiveTail = next
		if err := storeManifest(e.rootDir, updatedManifest); err != nil {
			return nil, nil, fmt.Errorf("checkpoint embedded engine: store rotated tail manifest: %w", err)
		}
		e.manifest = updatedManifest
		e.setActiveTailMetricsLocked()
	}
	return snapshot, retiredBefore, nil
}

// commitCheckpointLocked publishes a written checkpoint and removes the tails and
// segments it supersedes.
func (e *Engine) commitCheckpointLocked(meta CheckpointMeta, retiredBefore map[string]struct{}) error {
	updatedManifest := e.manifest
	updatedManifest.Checkpoints = append(append([]CheckpointMeta(nil), updatedManifest.Checkpoints...), meta)
	updatedManifest.ActiveCheckpoint = meta
	updatedManifest.PreviousTails = nil
	if e.tail != nil {
		updatedManifest.ActiveTail = e.tail.meta
	}
	var removable []string
	keptRetired := make([]RetiredSegment, 0, len(updatedManifest.RetiredSegments))
	for _, retired := range updatedManifest.RetiredSegments {
		if _, ok := retiredBefore[retired.ID]; ok {
			removable = append(removable, retired.ID)
			continue
		}
		keptRetired = append(keptRetired, retired)
	}
	updatedManifest.RetiredSegments = keptRetired
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("checkpoint embedded engine: store manifest: %w", err)
	}
	e.manifest = updatedManifest

	if err := removeSegmentDirs(e.rootDir, removable); err != nil {
		return fmt.Errorf("checkpoint embedded engine: remove retired segments: %w", err)
	}
	if err := e.pruneCheckpointHistoryLocked(); err != nil {
		return fmt.Errorf("checkpoint embedded engine: prune checkpoint history: %w", err)
	}
	e.logStaleTailCleanupWarning(pruneStaleTailJournals(e.rootDir, e.manifest.ActiveTail.ID))
	e.scheduleCompactionLocked()
	e.setActiveTailMetricsLocked()
	return nil
}

// scheduleCheckpointLocked starts a background checkpoint unless one is running.
func (e *Engine) scheduleCheckpointLocked() {
	if e == nil || e.closed || e.checkpointRunning {
		return
	}
	e.checkpointRunning = true
	e.backgroundTasks.Add(1)
	go func() {
		defer e.backgroundTasks.Done()
		if err := e.Checkpoint(context.Background()); err != nil {
			e.logAutoFlushError("tail-triggered checkpoint failed", err)
		}
		e.mu.Lock()
		e.checkpointRunning = false
		e.mu.Unlock()
	}()
}

func removeSegmentDirs(rootDir string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	segmentsRoot := filepath.Join(rootDir, segmentsDirName)
	for _, id := range ids {
		if err := os.RemoveAll(filepath.Join(segmentsRoot, id)); err != nil {
			return fmt.Errorf("remove segment %q: %w", id, err)
		}
	}
	if err := syncPath(segmentsRoot); err != nil {
		return fmt.Errorf("sync segments dir: %w", err)
	}
	return nil
}

func (e *Engine) applyRetentionLocked(ctx context.Context, retentionCutoffTimestamp int64) error {
	if e == nil || retentionCutoffTimestamp <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	updatedManifest, err := pruneExpiredCheckpoints(e.rootDir, e.manifest, retentionCutoffTimestamp)
	if err != nil {
		return err
	}
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return err
	}
	e.manifest = updatedManifest

	if err := e.applySegmentRetentionLocked(ctx, retentionCutoffTimestamp); err != nil {
		return err
	}
	e.hot = rebuildHotStoreWithRetention(e.hot, e.metrics, retentionCutoffTimestamp)
	e.refreshQueryPlanner()
	e.metrics.SetActiveSegments(len(e.segmentReaders))
	if e.queryExec != nil {
		e.queryExec.recentEventCacheMu.Lock()
		e.queryExec.pruneRecentEventTimelineCacheLocked(retentionCutoffTimestamp)
		e.queryExec.recentEventCacheMu.Unlock()
	}
	return nil
}

func (e *Engine) pruneCheckpointHistoryLocked() error {
	if e == nil {
		return nil
	}

	updatedManifest, err := pruneCheckpointHistory(e.rootDir, e.manifest, e.config.CheckpointRetentionCount)
	if err != nil {
		return err
	}
	e.manifest = updatedManifest
	return nil
}

func pruneCheckpointHistory(rootDir string, manifest Manifest, retentionCount int) (Manifest, error) {
	if retentionCount <= 0 || len(manifest.Checkpoints) <= retentionCount {
		return manifest, nil
	}

	sortedCheckpoints := append([]CheckpointMeta(nil), manifest.Checkpoints...)
	sort.Slice(sortedCheckpoints, func(i, j int) bool {
		if sortedCheckpoints[i].HighWaterMark != sortedCheckpoints[j].HighWaterMark {
			return sortedCheckpoints[i].HighWaterMark < sortedCheckpoints[j].HighWaterMark
		}
		return sortedCheckpoints[i].ID < sortedCheckpoints[j].ID
	})

	keptCheckpoints := append([]CheckpointMeta(nil), sortedCheckpoints[len(sortedCheckpoints)-retentionCount:]...)
	keptCheckpointIDs := make(map[string]struct{}, len(keptCheckpoints))
	for _, checkpoint := range keptCheckpoints {
		keptCheckpointIDs[checkpoint.ID] = struct{}{}
	}

	if manifest.ActiveCheckpoint.ID != "" {
		if _, exists := keptCheckpointIDs[manifest.ActiveCheckpoint.ID]; !exists {
			for i := range sortedCheckpoints {
				if sortedCheckpoints[i].ID != manifest.ActiveCheckpoint.ID {
					continue
				}
				keptCheckpoints = append(keptCheckpoints[1:], sortedCheckpoints[i])
				keptCheckpointIDs = make(map[string]struct{}, len(keptCheckpoints))
				for _, checkpoint := range keptCheckpoints {
					keptCheckpointIDs[checkpoint.ID] = struct{}{}
				}
				break
			}
		}
	}

	for _, checkpoint := range sortedCheckpoints {
		if _, keep := keptCheckpointIDs[checkpoint.ID]; keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(rootDir, checkpointsDirName, checkpoint.ID)); err != nil {
			return manifest, fmt.Errorf("remove checkpoint %q: %w", checkpoint.ID, err)
		}
	}

	sort.Slice(keptCheckpoints, func(i, j int) bool {
		if keptCheckpoints[i].HighWaterMark != keptCheckpoints[j].HighWaterMark {
			return keptCheckpoints[i].HighWaterMark < keptCheckpoints[j].HighWaterMark
		}
		return keptCheckpoints[i].ID < keptCheckpoints[j].ID
	})

	updatedManifest := manifest
	updatedManifest.Checkpoints = keptCheckpoints
	if len(keptCheckpoints) > 0 {
		updatedManifest.ActiveCheckpoint = latestCheckpointMeta(keptCheckpoints)
	}
	if err := storeManifest(rootDir, updatedManifest); err != nil {
		return manifest, err
	}

	return updatedManifest, nil
}

func (e *Engine) shouldCheckpointTailLocked() bool {
	if e == nil || e.tail == nil {
		return false
	}
	if e.config.CheckpointMaxTailEvents > 0 && e.tail.meta.EventCount > e.config.CheckpointMaxTailEvents {
		return true
	}
	if e.config.CheckpointMaxTailBytes > 0 && e.tail.meta.SizeBytes > e.config.CheckpointMaxTailBytes {
		return true
	}
	return false
}

func (e *Engine) logStaleTailCleanupWarning(err error) {
	if e == nil || err == nil {
		return
	}
	if e.logger == nil {
		e.logger = logging.GetLogger("embedded.engine")
	}

	e.logger.WarnWithFields(
		"embedded tail cleanup skipped after successful persistence",
		logging.Field("active_tail_id", e.manifest.ActiveTail.ID),
		logging.Field("error", err.Error()),
	)
}
