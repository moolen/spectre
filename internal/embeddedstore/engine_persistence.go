package embeddedstore

import (
	"context"
	"fmt"
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
	updatedManifest.ActiveSegments = append(updatedManifest.ActiveSegments, SegmentMeta{
		ID:            meta.ID,
		HighWaterMark: e.nextHighWaterMark,
	})
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

	start := time.Now()
	meta, err := writeCheckpoint(e.rootDir, e.projection, e.nextHighWaterMark)
	if err != nil {
		e.metrics.RecordCheckpoint(time.Since(start), err)
		return fmt.Errorf("checkpoint embedded engine: write checkpoint: %w", err)
	}

	updatedManifest := e.manifest
	updatedManifest.Checkpoints = append(updatedManifest.Checkpoints, meta)
	updatedManifest.ActiveCheckpoint = meta
	if e.tail != nil {
		updatedManifest.ActiveTail = e.tail.meta
	}
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		e.metrics.RecordCheckpoint(time.Since(start), err)
		return fmt.Errorf("checkpoint embedded engine: store manifest: %w", err)
	}

	e.manifest = updatedManifest
	if e.tail != nil {
		previousMeta := e.tail.meta
		nextTailMeta, err := e.tail.Rotate(e.nextHighWaterMark)
		if err != nil {
			e.metrics.RecordCheckpoint(time.Since(start), err)
			return fmt.Errorf("checkpoint embedded engine: rotate tail journal: %w", err)
		}

		updatedManifest.ActiveTail = nextTailMeta
		if err := storeManifest(e.rootDir, updatedManifest); err != nil {
			reopenedTail, reopenErr := openTailJournal(e.rootDir, previousMeta)
			if reopenErr == nil {
				_ = e.tail.Close()
				e.tail = reopenedTail
			}
			e.metrics.RecordCheckpoint(time.Since(start), err)
			return fmt.Errorf("checkpoint embedded engine: store rotated tail manifest: %w", err)
		}
		e.manifest = updatedManifest
	}
	e.metrics.RecordCheckpoint(time.Since(start), nil)
	return nil
}
