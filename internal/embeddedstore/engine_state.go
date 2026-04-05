package embeddedstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

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

func loadEngineState(rootDir string, manifest Manifest) ([]*segmentReader, *tailJournal, uint64, *Projection, *hotStore, startupMode, error) {
	readers := make([]*segmentReader, 0, len(manifest.ActiveSegments))
	for i := range manifest.ActiveSegments {
		segmentMeta := manifest.ActiveSegments[i]
		reader, err := openSegmentReader(rootDir, segmentBundleMeta{ID: segmentMeta.ID})
		if err != nil {
			return nil, nil, 0, nil, nil, startupModeRepair, fmt.Errorf("open active segment %q: %w", segmentMeta.ID, err)
		}
		readers = append(readers, reader)
	}

	checkpointProjection, checkpointHighWaterMark := loadStartupCheckpoint(rootDir, manifest)
	projection, tail, recoveredHot, recoveredHighWaterMark, ok, err := tryLoadFastStartupState(rootDir, manifest, checkpointProjection, checkpointHighWaterMark)
	if err != nil {
		return nil, nil, 0, nil, nil, startupModeRepair, err
	}
	if ok {
		return readers, tail, recoveredHighWaterMark, projection, recoveredHot, startupModeFast, nil
	}

	projection, recoveredHighWaterMark, err = buildRepairProjection(readers, manifest.ActiveSegments, checkpointProjection, checkpointHighWaterMark)
	if err != nil {
		return nil, nil, 0, nil, nil, startupModeRepair, err
	}

	return readers, nil, recoveredHighWaterMark, projection, nil, startupModeRepair, nil
}

func loadStartupCheckpoint(rootDir string, manifest Manifest) (*Projection, uint64) {
	checkpointMeta := manifest.ActiveCheckpoint
	if checkpointMeta.ID == "" && len(manifest.Checkpoints) > 0 {
		checkpointMeta = latestCheckpointMeta(manifest.Checkpoints)
	}
	if checkpointMeta.ID == "" {
		return nil, 0
	}

	projection, checkpointHighWaterMark, err := loadCheckpoint(rootDir, checkpointMeta)
	if err != nil {
		return nil, 0
	}

	return projection, checkpointHighWaterMark
}

func tryLoadFastStartupState(
	rootDir string,
	manifest Manifest,
	projection *Projection,
	checkpointHighWaterMark uint64,
) (*Projection, *tailJournal, *hotStore, uint64, bool, error) {
	if projection == nil {
		return nil, nil, nil, 0, false, nil
	}
	maxSegmentHighWaterMark := maxSegmentHighWaterMark(manifest.ActiveSegments)

	hot := newHotStore(HotStoreConfig{}, nil)
	if manifest.ActiveTail.ID == "" {
		if maxUint64(manifest.FlushHighWaterMark, maxSegmentHighWaterMark) > checkpointHighWaterMark {
			return nil, nil, nil, 0, false, nil
		}
		return projection, nil, hot, checkpointHighWaterMark, true, nil
	}
	if maxSegmentHighWaterMark > maxUint64(checkpointHighWaterMark, manifest.ActiveTail.LastHighWaterMark) {
		return nil, nil, nil, 0, false, nil
	}

	tail, err := openTailJournal(rootDir, manifest.ActiveTail)
	if err != nil {
		return nil, nil, nil, 0, false, fmt.Errorf("open active tail journal %q: %w", manifest.ActiveTail.ID, err)
	}
	if err := recoverTailState(projection, hot, tail, checkpointHighWaterMark); err != nil {
		_ = tail.Close()
		return nil, nil, nil, 0, false, err
	}

	return projection, tail, hot, maxUint64(checkpointHighWaterMark, tail.meta.LastHighWaterMark), true, nil
}

func buildRepairProjection(
	readers []*segmentReader,
	segments []SegmentMeta,
	checkpointProjection *Projection,
	checkpointHighWaterMark uint64,
) (*Projection, uint64, error) {
	replayReaders := make([]replaySegmentReader, 0, len(readers))
	for i := range readers {
		if i < len(segments) && segments[i].HighWaterMark <= checkpointHighWaterMark {
			continue
		}
		if readers[i].meta.EventCount == 0 {
			continue
		}

		replayReaders = append(replayReaders, replaySegmentReader{
			segmentID:      readers[i].meta.ID,
			reader:         readers[i],
			startTimestamp: readers[i].meta.MinTimestamp,
			endTimestamp:   readers[i].meta.MaxTimestamp,
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
