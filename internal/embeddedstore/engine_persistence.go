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
	meta, err := writeSegment(e.rootDir, segmentID, batch.Events)
	if err != nil {
		return fmt.Errorf("flush embedded engine: write segment: %w", err)
	}

	reader, err := openSegmentReader(e.rootDir, meta)
	if err != nil {
		return fmt.Errorf("flush embedded engine: open segment reader: %w", err)
	}

	updatedManifest := e.manifest
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
	e.refreshQueryPlanner()
	e.metrics.SetActiveSegments(len(e.segmentReaders))
	return nil
}

func (e *Engine) shouldFlushHotBySizeLocked() bool {
	if e.config.SegmentTargetBytes <= 0 {
		return false
	}

	batch := e.hot.ExtractFlushBatch(0)
	if len(batch.Events) == 0 {
		return false
	}

	return estimatedEventsSizeBytes(batch.Events) >= e.config.SegmentTargetBytes
}

func estimatedEventsSizeBytes(events []models.Event) int64 {
	var total int64
	for i := range events {
		// Estimate serialized footprint; exact framing size is unnecessary for threshold triggering.
		total += int64(len(events[i].ID) + len(events[i].Type))
		total += int64(len(events[i].Resource.UID) + len(events[i].Resource.Namespace) + len(events[i].Resource.Kind))
		total += int64(len(events[i].Resource.Name) + len(events[i].Resource.Group) + len(events[i].Resource.Version))
		total += int64(len(events[i].Data))
		total += 32
	}

	return total
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

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}

	return e.checkpointLocked(ctx)
}

func (e *Engine) checkpointLocked(ctx context.Context) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !e.ready.Load() {
		return nil
	}

	start := time.Now()
	defer func() {
		e.metrics.RecordCheckpoint(time.Since(start), err)
	}()

	meta, err := writeCheckpoint(e.rootDir, e.projection, e.nextHighWaterMark)
	if err != nil {
		return fmt.Errorf("checkpoint embedded engine: write checkpoint: %w", err)
	}

	updatedManifest := e.manifest
	updatedManifest.Checkpoints = append(updatedManifest.Checkpoints, meta)
	updatedManifest.ActiveCheckpoint = meta
	if e.tail != nil {
		updatedManifest.ActiveTail = e.tail.meta
	}
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("checkpoint embedded engine: store manifest: %w", err)
	}

	e.manifest = updatedManifest
	if e.tail != nil {
		previousMeta := e.tail.meta
		nextTailMeta, rotateErr := e.tail.Rotate(e.nextHighWaterMark)
		if rotateErr != nil {
			return fmt.Errorf("checkpoint embedded engine: rotate tail journal: %w", rotateErr)
		}

		updatedManifest.ActiveTail = nextTailMeta
		if err := storeManifest(e.rootDir, updatedManifest); err != nil {
			reopenedTail, reopenErr := openTailJournal(e.rootDir, previousMeta)
			if reopenErr == nil {
				_ = e.tail.Close()
				e.tail = reopenedTail
			}
			return fmt.Errorf("checkpoint embedded engine: store rotated tail manifest: %w", err)
		}
		e.manifest = updatedManifest
	}
	if err := e.pruneCheckpointHistoryLocked(); err != nil {
		return fmt.Errorf("checkpoint embedded engine: prune checkpoint history: %w", err)
	}

	e.setActiveTailMetricsLocked()
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
