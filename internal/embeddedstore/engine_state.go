package embeddedstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func embeddedRootDir(dataDir string) string {
	return filepath.Join(dataDir, "embedded")
}

type startupMode int

const (
	startupModeRepair startupMode = iota
	startupModeFast
)

const associatedIndexGeneration = 1

func (m startupMode) String() string {
	switch m {
	case startupModeFast:
		return "fast"
	default:
		return "repair"
	}
}

func loadEngineState(rootDir string, manifest Manifest) ([]*segmentReader, *tailJournal, uint64, *Projection, *hotStore, startupMode, int, error) {
	logger := logging.GetLogger("embedded.engine")

	segmentLoadStart := time.Now()
	readers, err := openActiveSegmentReaders(rootDir, manifest.ActiveSegments)
	if err != nil {
		return nil, nil, 0, nil, nil, startupModeRepair, 0, err
	}
	logger.DebugWithFields(
		"embedded startup segments loaded",
		logging.Field("active_segments", len(readers)),
		logging.Field("duration_ms", time.Since(segmentLoadStart).Milliseconds()),
	)

	checkpointLoadStart := time.Now()
	checkpointProjection, checkpointHighWaterMark, err := loadStartupCheckpoint(rootDir, manifest)
	if err != nil {
		return nil, nil, 0, nil, nil, startupModeRepair, 0, err
	}
	logger.DebugWithFields(
		"embedded startup checkpoint loaded",
		logging.Field("checkpoint_id", manifest.ActiveCheckpoint.ID),
		logging.Field("checkpoint_high_water_mark", checkpointHighWaterMark),
		logging.Field("duration_ms", time.Since(checkpointLoadStart).Milliseconds()),
	)

	fastPathStart := time.Now()
	projection, tail, recoveredHot, recoveredHighWaterMark, replayedTailEvents, ok, err := tryLoadFastStartupState(rootDir, manifest, checkpointProjection, checkpointHighWaterMark)
	if err != nil {
		return nil, nil, 0, nil, nil, startupModeRepair, 0, err
	}
	logger.DebugWithFields(
		"embedded startup fast-path recovery attempted",
		logging.Field("ok", ok),
		logging.Field("replayed_tail_events", replayedTailEvents),
		logging.Field("duration_ms", time.Since(fastPathStart).Milliseconds()),
	)
	if ok {
		return readers, tail, recoveredHighWaterMark, projection, recoveredHot, startupModeFast, replayedTailEvents, nil
	}

	repairBuildStart := time.Now()
	projection, recoveredHighWaterMark, err = buildRepairProjection(readers, manifest.ActiveSegments, checkpointProjection, checkpointHighWaterMark)
	if err != nil {
		return nil, nil, 0, nil, nil, startupModeRepair, 0, err
	}
	logger.InfoWithFields(
		"embedded startup repair projection built",
		logging.Field("duration_ms", time.Since(repairBuildStart).Milliseconds()),
		logging.Field("checkpoint_high_water_mark", checkpointHighWaterMark),
		logging.Field("recovered_high_water_mark", recoveredHighWaterMark),
	)

	return readers, nil, recoveredHighWaterMark, projection, nil, startupModeRepair, 0, nil
}

func openActiveSegmentReaders(rootDir string, segments []SegmentMeta) ([]*segmentReader, error) {
	readers := make([]*segmentReader, 0, len(segments))
	for i := range segments {
		segmentMeta := segments[i]
		reader, err := openSegmentReader(rootDir, segmentMeta.bundleMeta())
		if err != nil {
			return nil, fmt.Errorf("open active segment %q: %w", segmentMeta.ID, err)
		}
		readers = append(readers, reader)
	}
	return readers, nil
}

func migrateSegmentIndexGeneration(rootDir string, manifest Manifest, logger *logging.Logger) (Manifest, []*segmentReader, int, error) {
	if manifest.SegmentIndexGeneration >= associatedIndexGeneration || len(manifest.ActiveSegments) == 0 {
		return manifest, nil, 0, nil
	}
	if logger == nil {
		logger = logging.GetLogger("embedded.engine")
	}

	logger.InfoWithFields(
		"embedded startup segment index migration started in background",
		logging.Field("active_segments", len(manifest.ActiveSegments)),
	)

	readers, err := openActiveSegmentReaders(rootDir, manifest.ActiveSegments)
	if err != nil {
		return manifest, nil, 0, fmt.Errorf("open startup migration segments: %w", err)
	}

	start := time.Now()
	migratedSegments := make([]SegmentMeta, 0, len(manifest.ActiveSegments))
	rewrittenCount := 0

	for i := range manifest.ActiveSegments {
		currentMeta := manifest.ActiveSegments[i]
		reader := readers[i]

		hasAssociatedIndex, err := reader.HasAssociatedIndex()
		if err != nil {
			return manifest, nil, 0, fmt.Errorf("inspect startup migration segment %q: %w", currentMeta.ID, err)
		}
		if hasAssociatedIndex {
			migratedSegments = append(migratedSegments, currentMeta)
			continue
		}

		bundleMeta, err := reader.EnsureBundleMeta()
		if err != nil {
			return manifest, nil, 0, fmt.Errorf("load startup migration segment %q metadata: %w", currentMeta.ID, err)
		}

		events, err := reader.ScanTimeRange(context.Background(), bundleMeta.MinTimestamp, bundleMeta.MaxTimestamp)
		if err != nil {
			return manifest, nil, 0, fmt.Errorf("scan startup migration segment %q: %w", currentMeta.ID, err)
		}

		replacementID := newSegmentID(currentMeta.HighWaterMark)
		replacementMeta, err := writeSegment(rootDir, replacementID, events)
		if err != nil {
			return manifest, nil, 0, fmt.Errorf("rewrite startup migration segment %q: %w", currentMeta.ID, err)
		}

		migratedSegments = append(migratedSegments, segmentMetaFromBundle(replacementMeta, currentMeta.HighWaterMark))
		rewrittenCount++
	}

	if rewrittenCount == 0 {
		manifest.SegmentIndexGeneration = associatedIndexGeneration
		updatedReaders, err := openActiveSegmentReaders(rootDir, manifest.ActiveSegments)
		if err != nil {
			return manifest, nil, 0, fmt.Errorf("open startup migration current readers: %w", err)
		}
		logger.InfoWithFields(
			"embedded startup segment index migration skipped rewrite and marked current generation",
			logging.Field("active_segments", len(manifest.ActiveSegments)),
			logging.Field("duration_ms", time.Since(start).Milliseconds()),
		)
		return manifest, updatedReaders, 0, nil
	}

	updatedManifest := manifest
	updatedManifest.ActiveSegments = migratedSegments
	updatedManifest.SegmentIndexGeneration = associatedIndexGeneration
	updatedReaders, err := openActiveSegmentReaders(rootDir, updatedManifest.ActiveSegments)
	if err != nil {
		return manifest, nil, 0, fmt.Errorf("open startup migration replacement readers: %w", err)
	}

	logger.InfoWithFields(
		"embedded startup segment index migration completed",
		logging.Field("rewritten_segments", rewrittenCount),
		logging.Field("active_segments", len(updatedManifest.ActiveSegments)),
		logging.Field("legacy_segments_retained", rewrittenCount),
		logging.Field("duration_ms", time.Since(start).Milliseconds()),
	)
	return updatedManifest, updatedReaders, rewrittenCount, nil
}

func loadStartupCheckpoint(rootDir string, manifest Manifest) (*Projection, uint64, error) {
	checkpointMeta := manifest.ActiveCheckpoint
	if checkpointMeta.ID == "" && len(manifest.Checkpoints) > 0 {
		checkpointMeta = latestCheckpointMeta(manifest.Checkpoints)
	}
	if checkpointMeta.ID == "" {
		return nil, 0, nil
	}

	projection, checkpointHighWaterMark, err := loadCheckpoint(rootDir, checkpointMeta)
	if err != nil {
		return nil, 0, err
	}

	return projection, checkpointHighWaterMark, nil
}

func tryLoadFastStartupState(
	rootDir string,
	manifest Manifest,
	projection *Projection,
	checkpointHighWaterMark uint64,
) (*Projection, *tailJournal, *hotStore, uint64, int, bool, error) {
	logger := logging.GetLogger("embedded.engine")
	if projection == nil {
		return nil, nil, nil, 0, 0, false, nil
	}
	maxSegmentHighWaterMark := maxSegmentHighWaterMark(manifest.ActiveSegments)

	hot := newHotStore(HotStoreConfig{}, nil)
	if manifest.ActiveTail.ID == "" {
		if maxUint64(manifest.FlushHighWaterMark, maxSegmentHighWaterMark) > checkpointHighWaterMark {
			return nil, nil, nil, 0, 0, false, nil
		}
		return projection, nil, hot, checkpointHighWaterMark, 0, true, nil
	}
	if maxSegmentHighWaterMark > maxUint64(checkpointHighWaterMark, manifest.ActiveTail.LastHighWaterMark) {
		return nil, nil, nil, 0, 0, false, nil
	}

	tail, err := openTailJournal(rootDir, manifest.ActiveTail)
	if err != nil {
		logger.WarnWithFields(
			"embedded startup fast path falling back to repair after tail journal open failure",
			logging.Field("tail_id", manifest.ActiveTail.ID),
			logging.Field("error", err.Error()),
		)
		return nil, nil, nil, 0, 0, false, nil
	}
	replayedTailEvents, err := recoverTailState(projection, hot, tail, checkpointHighWaterMark)
	if err != nil {
		_ = tail.Close()
		logger.WarnWithFields(
			"embedded startup fast path falling back to repair after tail replay failure",
			logging.Field("tail_id", manifest.ActiveTail.ID),
			logging.Field("checkpoint_high_water_mark", checkpointHighWaterMark),
			logging.Field("error", err.Error()),
		)
		return nil, nil, nil, 0, 0, false, nil
	}

	return projection, tail, hot, maxUint64(checkpointHighWaterMark, tail.meta.LastHighWaterMark), replayedTailEvents, true, nil
}

func buildRepairProjection(
	readers []*segmentReader,
	segments []SegmentMeta,
	checkpointProjection *Projection,
	checkpointHighWaterMark uint64,
) (*Projection, uint64, error) {
	replayReaders := make([]replaySegmentReader, 0, len(readers))
	for i := range readers {
		meta, err := readers[i].EnsureBundleMeta()
		if err != nil {
			return nil, 0, fmt.Errorf("load active segment %q metadata: %w", segments[i].ID, err)
		}
		if i < len(segments) && segments[i].HighWaterMark <= checkpointHighWaterMark {
			continue
		}
		if meta.EventCount == 0 {
			continue
		}

		replayReaders = append(replayReaders, replaySegmentReader{
			segmentID:      meta.ID,
			reader:         readers[i],
			startTimestamp: meta.MinTimestamp,
			endTimestamp:   meta.MaxTimestamp,
		})
	}

	if len(replayReaders) == 0 {
		if checkpointProjection != nil {
			return checkpointProjection, maxUint64(checkpointHighWaterMark, maxSegmentHighWaterMark(segments)), nil
		}
		return NewProjection(), maxSegmentHighWaterMark(segments), nil
	}

	if checkpointProjection != nil {
		if err := replaySegmentReaders(context.Background(), checkpointProjection, replayReaders); err != nil {
			return nil, 0, err
		}
		return checkpointProjection, maxUint64(checkpointHighWaterMark, maxSegmentHighWaterMark(segments)), nil
	}

	if applyProjectionEventUsesDefaultImplementation() {
		projection, err := buildProjectionFromReplayReaders(context.Background(), replayReaders)
		if err != nil {
			return nil, 0, err
		}
		return projection, maxSegmentHighWaterMark(segments), nil
	}

	projection := NewProjection()
	if err := replaySegmentReaders(context.Background(), projection, replayReaders); err != nil {
		return nil, 0, err
	}

	return projection, maxSegmentHighWaterMark(segments), nil
}

func sortReplayEvents(events []models.Event) {
	sort.SliceStable(events, func(i, j int) bool {
		return compareEventOrder(events[i], events[j]) < 0
	})
}

func replayProjectionEvents(projection *Projection, events []models.Event) error {
	for i := range events {
		if err := applyProjectionEvent(projection, events[i]); err != nil {
			return fmt.Errorf("open embedded engine: replay event %d: %w", i, err)
		}
	}

	return nil
}

func latestCheckpointMeta(checkpoints []CheckpointMeta) CheckpointMeta {
	latest := checkpoints[0]
	for i := 1; i < len(checkpoints); i++ {
		if checkpoints[i].HighWaterMark >= latest.HighWaterMark {
			latest = checkpoints[i]
		}
	}

	return latest
}

func maxSegmentHighWaterMark(segments []SegmentMeta) uint64 {
	var maxValue uint64
	for i := range segments {
		if segments[i].HighWaterMark > maxValue {
			maxValue = segments[i].HighWaterMark
		}
	}

	return maxValue
}

func maxUint64(values ...uint64) uint64 {
	var maxValue uint64
	for i := range values {
		if values[i] > maxValue {
			maxValue = values[i]
		}
	}

	return maxValue
}

func newSegmentID(highWaterMark uint64) string {
	return fmt.Sprintf("seg-%020d-%d", highWaterMark, time.Now().UTC().UnixNano())
}
