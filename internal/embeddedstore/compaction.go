package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// Compaction merges runs of adjacent small segments into larger ones. Segments are never
// merged past compactionMaxMergedBytes or compactionMaxMergedSpan, so the store stays a
// sequence of bounded, time-ordered segments: retention can drop whole segments, and no
// single rewrite ever has to touch the full history.
var (
	compactionMaxMergedBytes int64 = 512 << 20
	compactionMaxMergedSpan        = 24 * time.Hour
)

type compactionPlan struct {
	segments []SegmentMeta
}

func (p *compactionPlan) ids() []string {
	ids := make([]string, len(p.segments))
	for i := range p.segments {
		ids[i] = p.segments[i].ID
	}
	return ids
}

// Compact merges every eligible run of segments synchronously.
func (e *Engine) Compact(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("compact embedded engine: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		e.compactionMu.Lock()
		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			e.compactionMu.Unlock()
			return nil
		}
		plan := e.planCompactionLocked(2)
		e.mu.Unlock()
		if plan == nil {
			e.compactionMu.Unlock()
			return nil
		}
		merged, err := e.executeCompaction(ctx, plan)
		e.compactionMu.Unlock()
		if err != nil {
			return err
		}
		if !merged {
			return nil
		}
	}
}

func (e *Engine) shouldAutoCompactLocked() bool {
	if e == nil || e.config.DisableAutoCompaction {
		return false
	}
	return e.planCompactionLocked(e.compactionMinSegments()) != nil
}

func (e *Engine) compactionMinSegments() int {
	minSegments := e.config.CompactionMinSegments
	if minSegments < 2 {
		minSegments = 2
	}
	return minSegments
}

// scheduleCompactionLocked starts a background compaction when enough small segments
// have accumulated. Ingest keeps running while records are merged; only the final
// manifest swap takes the engine lock.
func (e *Engine) scheduleCompactionLocked() {
	if e == nil || e.closed || e.config.DisableAutoCompaction || e.compactionRunning {
		return
	}
	plan := e.planCompactionLocked(e.compactionMinSegments())
	if plan == nil {
		return
	}
	e.compactionRunning = true
	e.backgroundTasks.Add(1)
	go func() {
		defer e.backgroundTasks.Done()
		e.compactionMu.Lock()
		_, err := e.executeCompaction(context.Background(), plan)
		e.compactionMu.Unlock()
		if err != nil {
			e.logAutoFlushError("background compaction failed", err)
		}
		e.mu.Lock()
		e.compactionRunning = false
		e.scheduleCompactionLocked()
		e.mu.Unlock()
	}()
}

// planCompactionLocked returns the first run of at least minRun adjacent segments that
// can be merged without exceeding the size and time-span limits.
func (e *Engine) planCompactionLocked(minRun int) *compactionPlan {
	segments := e.manifest.ActiveSegments
	var run []SegmentMeta
	var runBytes int64
	var runMin, runMax int64

	flush := func() *compactionPlan {
		if len(run) >= minRun {
			return &compactionPlan{segments: append([]SegmentMeta(nil), run...)}
		}
		return nil
	}

	for i := range segments {
		segment := segments[i]
		size := e.segmentSizeBytes(segment)
		eligible := size < compactionMaxMergedBytes/2
		if eligible && len(run) > 0 {
			newMin, newMax := min(runMin, segment.MinTimestamp), max(runMax, segment.MaxTimestamp)
			if runBytes+size > compactionMaxMergedBytes || time.Duration(newMax-newMin) > compactionMaxMergedSpan {
				if plan := flush(); plan != nil {
					return plan
				}
				run, runBytes = nil, 0
			}
		}
		if !eligible {
			if plan := flush(); plan != nil {
				return plan
			}
			run, runBytes = nil, 0
			continue
		}
		if len(run) == 0 {
			runMin, runMax = segment.MinTimestamp, segment.MaxTimestamp
		} else {
			runMin, runMax = min(runMin, segment.MinTimestamp), max(runMax, segment.MaxTimestamp)
		}
		run = append(run, segment)
		runBytes += size
	}
	return flush()
}

func (e *Engine) segmentSizeBytes(segment SegmentMeta) int64 {
	if segment.SizeBytes > 0 {
		return segment.SizeBytes
	}
	info, err := os.Stat(segmentEventsPath(e.rootDir, segment.ID))
	if err != nil {
		return compactionMaxMergedBytes
	}
	return info.Size()
}

// executeCompaction merges the plan's segments into one new segment and swaps it into
// the manifest. It returns false when the plan went stale while merging.
func (e *Engine) executeCompaction(ctx context.Context, plan *compactionPlan) (merged bool, err error) {
	start := time.Now()
	defer func() {
		if merged || err != nil {
			e.metrics.RecordCompaction(time.Since(start), err)
		}
	}()

	var highWaterMark uint64
	paths := make([]string, 0, len(plan.segments))
	for i := range plan.segments {
		highWaterMark = max(highWaterMark, plan.segments[i].HighWaterMark)
		paths = append(paths, segmentEventsPath(e.rootDir, plan.segments[i].ID))
	}

	builder, err := newSegmentBuilder(e.rootDir, newSegmentID(highWaterMark))
	if err != nil {
		return false, fmt.Errorf("compact embedded engine: %w", err)
	}
	if err := mergeSegmentRecords(paths, func(record *rawSegmentRecord) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return builder.AppendRecord(record)
	}); err != nil {
		builder.Abort()
		return false, fmt.Errorf("compact embedded engine: merge segments: %w", err)
	}
	meta, err := builder.Finish()
	if err != nil {
		return false, fmt.Errorf("compact embedded engine: write compacted segment: %w", err)
	}
	newSegmentDir := filepath.Join(e.rootDir, segmentsDirName, meta.ID)

	e.mu.Lock()
	position, ok := e.locateSegmentRunLocked(plan)
	if e.closed || !ok {
		e.mu.Unlock()
		_ = os.RemoveAll(newSegmentDir)
		return false, nil
	}

	newReader, err := openSegmentReader(e.rootDir, meta)
	if err != nil {
		e.mu.Unlock()
		_ = os.RemoveAll(newSegmentDir)
		return false, fmt.Errorf("compact embedded engine: open compacted segment reader: %w", err)
	}

	updatedManifest := e.manifest
	updatedSegments := make([]SegmentMeta, 0, len(e.manifest.ActiveSegments)-len(plan.segments)+1)
	updatedSegments = append(updatedSegments, e.manifest.ActiveSegments[:position]...)
	updatedSegments = append(updatedSegments, segmentMetaFromBundle(meta, highWaterMark))
	updatedSegments = append(updatedSegments, e.manifest.ActiveSegments[position+len(plan.segments):]...)
	updatedManifest.ActiveSegments = updatedSegments
	for _, id := range plan.ids() {
		updatedManifest.RetiredSegments = append(updatedManifest.RetiredSegments, RetiredSegment{ID: id, ReplacedBy: []string{meta.ID}})
	}
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		e.mu.Unlock()
		_ = os.RemoveAll(newSegmentDir)
		return false, fmt.Errorf("compact embedded engine: store manifest: %w", err)
	}

	updatedReaders := make([]*segmentReader, 0, len(e.segmentReaders)-len(plan.segments)+1)
	updatedReaders = append(updatedReaders, e.segmentReaders[:position]...)
	updatedReaders = append(updatedReaders, newReader)
	updatedReaders = append(updatedReaders, e.segmentReaders[position+len(plan.segments):]...)

	e.manifest = updatedManifest
	e.segmentReaders = updatedReaders
	e.projection.AttachSegmentLocations(meta.ID, builder.Locations())
	e.projection.ReleaseSegmentHandles(plan.ids())
	e.refreshQueryPlanner()
	e.metrics.SetActiveSegments(len(e.segmentReaders))
	e.mu.Unlock()

	if e.logger != nil {
		e.logger.DebugWithFields(
			"embedded compaction merged segments",
			logging.Field("merged_segments", len(plan.segments)),
			logging.Field("events", meta.EventCount),
			logging.Field("bytes", meta.SizeBytes),
			logging.Field("duration_ms", time.Since(start).Milliseconds()),
		)
	}
	return true, nil
}

// locateSegmentRunLocked finds the plan's segments as a contiguous run in the manifest
// and checks that the readers line up with it.
func (e *Engine) locateSegmentRunLocked(plan *compactionPlan) (int, bool) {
	segments := e.manifest.ActiveSegments
	if len(segments) != len(e.segmentReaders) {
		return 0, false
	}
	for start := 0; start+len(plan.segments) <= len(segments); start++ {
		if segments[start].ID != plan.segments[0].ID {
			continue
		}
		for i := range plan.segments {
			if segments[start+i].ID != plan.segments[i].ID || e.segmentReaders[start+i].ID() != plan.segments[i].ID {
				return 0, false
			}
		}
		return start, true
	}
	return 0, false
}

// removeRetiredSegmentsLocked deletes segments retired before the checkpoint that was
// just written; nothing on disk references them any more.
func (e *Engine) removeRetiredSegmentsLocked() error {
	if len(e.manifest.RetiredSegments) == 0 {
		return nil
	}
	retired := e.manifest.RetiredSegments
	updatedManifest := e.manifest
	updatedManifest.RetiredSegments = nil
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("store manifest: %w", err)
	}
	e.manifest = updatedManifest

	segmentsRoot := filepath.Join(e.rootDir, segmentsDirName)
	for i := range retired {
		if err := os.RemoveAll(filepath.Join(segmentsRoot, retired[i].ID)); err != nil {
			return fmt.Errorf("remove retired segment %q: %w", retired[i].ID, err)
		}
	}
	if err := syncPath(segmentsRoot); err != nil {
		return fmt.Errorf("sync segments dir: %w", err)
	}
	return nil
}

// remapRetiredSegmentReferences moves version references restored from a checkpoint off
// segments that were replaced after that checkpoint was written.
func remapRetiredSegmentReferences(rootDir string, projection *Projection, retired []RetiredSegment) error {
	if projection == nil || len(retired) == 0 {
		return nil
	}
	for i := range retired {
		for _, replacement := range retired[i].ReplacedBy {
			locations, err := scanSegmentLocations(segmentEventsPath(rootDir, replacement))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("scan replacement segment %q: %w", replacement, err)
			}
			projection.AttachSegmentLocations(replacement, locations)
		}
	}
	ids := make([]string, 0, len(retired))
	for i := range retired {
		ids = append(ids, retired[i].ID)
	}
	projection.ReleaseSegmentHandles(ids)
	return nil
}

// attachSegmentsAfter points versions at the records of segments flushed after
// highWaterMark.
func attachSegmentsAfter(rootDir string, projection *Projection, segments []SegmentMeta, highWaterMark uint64) error {
	for _, segment := range segments {
		if segment.HighWaterMark <= highWaterMark {
			continue
		}
		locations, err := scanSegmentLocations(segmentEventsPath(rootDir, segment.ID))
		if err != nil {
			return fmt.Errorf("scan segment %q: %w", segment.ID, err)
		}
		projection.AttachSegmentLocations(segment.ID, locations)
	}
	return nil
}
