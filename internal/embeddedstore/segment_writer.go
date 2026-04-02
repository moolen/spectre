package embeddedstore

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/moolen/spectre/internal/models"
)

type segmentSortableRecord struct {
	event    models.Event
	sequence int
}

func writeSegment(rootDir, segmentID string, events []models.Event) (segmentBundleMeta, error) {
	if rootDir == "" {
		return segmentBundleMeta{}, fmt.Errorf("write segment: root dir is empty")
	}
	if segmentID == "" {
		return segmentBundleMeta{}, fmt.Errorf("write segment: segment id is empty")
	}

	segmentsRoot := filepath.Join(rootDir, segmentsDirName)
	if err := ensureDirWithParentSync(segmentsRoot); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: ensure segments dir: %w", err)
	}

	tmpRoot := filepath.Join(rootDir, segmentTempDirName)
	if err := ensureDirWithParentSync(tmpRoot); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: ensure temp dir: %w", err)
	}

	finalSegmentDir := filepath.Join(segmentsRoot, segmentID)
	if _, err := os.Stat(finalSegmentDir); err == nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: segment %q already exists", segmentID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return segmentBundleMeta{}, fmt.Errorf("write segment: stat existing segment path: %w", err)
	}

	tmpSegmentDir, err := os.MkdirTemp(tmpRoot, segmentID+".tmp-*")
	if err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: create temp segment dir: %w", err)
	}

	cleanupTmpDir := true
	defer func() {
		if cleanupTmpDir {
			_ = os.RemoveAll(tmpSegmentDir)
		}
	}()

	meta, err := writeSegmentBundle(tmpSegmentDir, segmentID, events)
	if err != nil {
		return segmentBundleMeta{}, err
	}

	if err := syncPath(tmpSegmentDir); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: sync temp segment dir: %w", err)
	}
	if err := os.Rename(tmpSegmentDir, finalSegmentDir); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: move segment bundle into place: %w", err)
	}
	if err := syncPath(segmentsRoot); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: sync segments dir: %w", err)
	}

	cleanupTmpDir = false
	return meta, nil
}

func ensureDirWithParentSync(path string) error {
	created, err := pathCreated(path)
	if err != nil {
		return fmt.Errorf("stat dir: %w", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if created {
		if err := syncPath(filepath.Dir(path)); err != nil {
			return fmt.Errorf("sync parent dir: %w", err)
		}
	}
	return nil
}

func writeSegmentBundle(segmentDir, segmentID string, events []models.Event) (segmentBundleMeta, error) {
	records := make([]segmentSortableRecord, len(events))
	for i := range events {
		records[i] = segmentSortableRecord{
			event:    events[i],
			sequence: i,
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].event.Timestamp != records[j].event.Timestamp {
			return records[i].event.Timestamp < records[j].event.Timestamp
		}
		return records[i].sequence < records[j].sequence
	})

	eventsPath := filepath.Join(segmentDir, segmentEventsFile)
	eventsFile, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: open events file: %w", err)
	}

	writeErr := func(stage string, err error) (segmentBundleMeta, error) {
		_ = eventsFile.Close()
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: %s: %w", stage, err)
	}

	timeIndexEntries := make([]segmentTimeIndexEntry, 0, len(records)/segmentIndexStride+1)
	resourceIndex := segmentResourceIndex{UIDOffsets: make(map[string][]int64)}
	dimensionSet := make(map[segmentDimensionEntry]struct{})
	namespaceKindSet := make(map[string]map[string]struct{})
	recordOffsets := make([]int64, 0, len(records))

	var offset int64
	var minTimestamp int64
	var maxTimestamp int64
	if len(records) > 0 {
		minTimestamp = records[0].event.Timestamp
		maxTimestamp = records[len(records)-1].event.Timestamp
	}

	for i := range records {
		record := records[i]
		recordOffsets = append(recordOffsets, offset)

		if i%segmentIndexStride == 0 {
			timeIndexEntries = append(timeIndexEntries, segmentTimeIndexEntry{
				Timestamp: record.event.Timestamp,
				Offset:    offset,
			})
		}

		framed, err := encodeFramedEvent(record.event)
		if err != nil {
			return writeErr("encode event", err)
		}

		written, err := writeAll(eventsFile, framed)
		if err != nil {
			return writeErr("write events payload", err)
		}
		offset += int64(written)

		uid := record.event.Resource.UID
		if uid != "" {
			resourceIndex.UIDOffsets[uid] = append(resourceIndex.UIDOffsets[uid], recordOffsets[len(recordOffsets)-1])
		}

		dimension := segmentDimensionEntry{
			Namespace: record.event.Resource.Namespace,
			Kind:      record.event.Resource.Kind,
		}
		dimensionSet[dimension] = struct{}{}

		if _, ok := namespaceKindSet[dimension.Namespace]; !ok {
			namespaceKindSet[dimension.Namespace] = make(map[string]struct{})
		}
		namespaceKindSet[dimension.Namespace][dimension.Kind] = struct{}{}
	}

	if len(records) > 0 {
		lastOffset := recordOffsets[len(recordOffsets)-1]
		lastRecord := records[len(records)-1]
		if len(timeIndexEntries) == 0 || timeIndexEntries[len(timeIndexEntries)-1].Offset != lastOffset {
			timeIndexEntries = append(timeIndexEntries, segmentTimeIndexEntry{
				Timestamp: lastRecord.event.Timestamp,
				Offset:    lastOffset,
			})
		}
	}

	if err := eventsFile.Sync(); err != nil {
		return writeErr("sync events file", err)
	}
	if err := eventsFile.Close(); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: close events file: %w", err)
	}

	namespaceKinds := make(map[string][]string, len(namespaceKindSet))
	for namespace, kindSet := range namespaceKindSet {
		kinds := make([]string, 0, len(kindSet))
		for kind := range kindSet {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		namespaceKinds[namespace] = kinds
	}
	namespaceKinds = normalizeNamespaceKinds(namespaceKinds)

	dimensions := make([]segmentDimensionEntry, 0, len(dimensionSet))
	for entry := range dimensionSet {
		dimensions = append(dimensions, entry)
	}
	sort.Slice(dimensions, func(i, j int) bool {
		if dimensions[i].Namespace != dimensions[j].Namespace {
			return dimensions[i].Namespace < dimensions[j].Namespace
		}
		return dimensions[i].Kind < dimensions[j].Kind
	})

	meta := segmentBundleMeta{
		ID:             segmentID,
		EventCount:     len(records),
		MinTimestamp:   minTimestamp,
		MaxTimestamp:   maxTimestamp,
		NamespaceKinds: namespaceKinds,
	}

	if err := writeTimeIndex(filepath.Join(segmentDir, segmentTimeIndexFile), timeIndexEntries); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: time index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(segmentDir, segmentUIDIndexFile), resourceIndex); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: uid index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(segmentDir, segmentDimIndexFile), segmentDimensionIndex{Entries: dimensions}); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: dimension index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(segmentDir, segmentStatsFile), meta); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: stats: %w", err)
	}

	return meta, nil
}

func writeTimeIndex(path string, entries []segmentTimeIndexEntry) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var entryBuf [16]byte
	for i := range entries {
		binary.BigEndian.PutUint64(entryBuf[:8], uint64(entries[i].Timestamp))
		binary.BigEndian.PutUint64(entryBuf[8:], uint64(entries[i].Offset))
		if _, err := writeAll(file, entryBuf[:]); err != nil {
			return fmt.Errorf("write entry %d: %w", i, err)
		}
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := writeAll(file, payload); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}

	return nil
}

func writeAll(writer io.Writer, payload []byte) (int, error) {
	written := 0
	for written < len(payload) {
		n, err := writer.Write(payload[written:])
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
		written += n
	}
	return written, nil
}
