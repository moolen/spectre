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

func loadEngineState(rootDir string, manifest Manifest) ([]*segmentReader, uint64, *Projection, error) {
	var checkpointProjection *Projection
	var checkpointHighWaterMark uint64
	if len(manifest.Checkpoints) > 0 {
		latestCheckpoint := latestCheckpointMeta(manifest.Checkpoints)
		var err error
		checkpointProjection, checkpointHighWaterMark, err = loadCheckpoint(rootDir, latestCheckpoint)
		if err != nil {
			return nil, 0, nil, err
		}
	}

	readers := make([]*segmentReader, 0, len(manifest.ActiveSegments))
	replayReaders := make([]replaySegmentReader, 0, len(manifest.ActiveSegments))
	for i := range manifest.ActiveSegments {
		segmentMeta := manifest.ActiveSegments[i]
		reader, err := openSegmentReader(rootDir, segmentBundleMeta{ID: segmentMeta.ID})
		if err != nil {
			return nil, 0, nil, fmt.Errorf("open active segment %q: %w", segmentMeta.ID, err)
		}
		readers = append(readers, reader)

		needsReplay := checkpointProjection == nil || segmentMeta.HighWaterMark > checkpointHighWaterMark
		if !needsReplay || reader.meta.EventCount == 0 {
			continue
		}

		replayReaders = append(replayReaders, replaySegmentReader{
			segmentID:      segmentMeta.ID,
			reader:         reader,
			startTimestamp: reader.meta.MinTimestamp,
			endTimestamp:   reader.meta.MaxTimestamp,
		})
	}

	projection := checkpointProjection
	if projection == nil {
		if len(replayReaders) == 0 {
			projection = NewProjection()
		} else if applyProjectionEventUsesDefaultImplementation() {
			var err error
			projection, err = buildProjectionFromReplayReaders(context.Background(), replayReaders)
			if err != nil {
				return nil, 0, nil, err
			}
		} else {
			projection = NewProjection()
			if err := replaySegmentReaders(context.Background(), projection, replayReaders); err != nil {
				return nil, 0, nil, err
			}
		}
	} else if err := replaySegmentReaders(context.Background(), projection, replayReaders); err != nil {
		return nil, 0, nil, err
	}

	return readers, checkpointHighWaterMark, projection, nil
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
