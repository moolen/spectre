package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (e *Engine) Compact(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("compact embedded engine: engine is nil")
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

	minSegments := e.config.CompactionMinSegments
	if minSegments < 2 {
		minSegments = 2
	}
	if len(e.manifest.ActiveSegments) < minSegments {
		return nil
	}
	start := time.Now()

	mergedEvents := make([]models.Event, 0)
	oldSegmentIDs := make([]string, 0, len(e.segmentReaders))
	for i := range e.segmentReaders {
		reader := e.segmentReaders[i]
		if reader == nil {
			continue
		}
		meta, err := reader.EnsureBundleMeta()
		if err != nil {
			e.metrics.RecordCompaction(time.Since(start), err)
			return fmt.Errorf("compact embedded engine: load segment %d metadata: %w", i, err)
		}
		oldSegmentIDs = append(oldSegmentIDs, meta.ID)
		if meta.EventCount == 0 {
			continue
		}
		events, err := reader.ScanTimeRange(ctx, meta.MinTimestamp, meta.MaxTimestamp)
		if err != nil {
			e.metrics.RecordCompaction(time.Since(start), err)
			return fmt.Errorf("compact embedded engine: scan segment %q: %w", meta.ID, err)
		}
		mergedEvents = append(mergedEvents, events...)
	}
	if len(oldSegmentIDs) < minSegments {
		return nil
	}

	newSegmentHighWaterMark := maxSegmentHighWaterMark(e.manifest.ActiveSegments)
	compactedSegmentID := newSegmentID(newSegmentHighWaterMark)
	meta, err := writeSegment(e.rootDir, compactedSegmentID, mergedEvents)
	if err != nil {
		e.metrics.RecordCompaction(time.Since(start), err)
		return fmt.Errorf("compact embedded engine: write compacted segment: %w", err)
	}
	newReader, err := openSegmentReader(e.rootDir, meta)
	if err != nil {
		e.metrics.RecordCompaction(time.Since(start), err)
		return fmt.Errorf("compact embedded engine: open compacted segment reader: %w", err)
	}

	updatedManifest := e.manifest
	updatedManifest.ActiveSegments = []SegmentMeta{
		segmentMetaFromBundle(meta, newSegmentHighWaterMark),
	}
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		e.metrics.RecordCompaction(time.Since(start), err)
		return fmt.Errorf("compact embedded engine: store manifest: %w", err)
	}

	e.manifest = updatedManifest
	e.segmentReaders = []*segmentReader{newReader}
	e.refreshQueryPlanner()
	e.metrics.SetActiveSegments(len(e.segmentReaders))

	segmentsRoot := filepath.Join(e.rootDir, segmentsDirName)
	for i := range oldSegmentIDs {
		if oldSegmentIDs[i] == meta.ID {
			continue
		}
		if err := os.RemoveAll(filepath.Join(segmentsRoot, oldSegmentIDs[i])); err != nil {
			e.metrics.RecordCompaction(time.Since(start), err)
			return fmt.Errorf("compact embedded engine: remove old segment %q: %w", oldSegmentIDs[i], err)
		}
	}
	if err := syncPath(segmentsRoot); err != nil {
		e.metrics.RecordCompaction(time.Since(start), err)
		return fmt.Errorf("compact embedded engine: sync segments dir: %w", err)
	}
	e.metrics.RecordCompaction(time.Since(start), nil)
	return nil
}
