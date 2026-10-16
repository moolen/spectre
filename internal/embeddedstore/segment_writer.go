package embeddedstore

import (
	"bufio"
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

// segmentRecordLocation identifies where a resource version's record was written, so the
// projection can drop the in-memory payload and read it back on demand.
type segmentRecordLocation struct {
	UID       string
	EventID   string
	Timestamp int64
	Offset    int64
	Length    uint32
}

func writeSegment(rootDir, segmentID string, events []models.Event) (segmentBundleMeta, error) {
	meta, _, err := writeSegmentLocated(rootDir, segmentID, events)
	return meta, err
}

// writeSegmentLocated sorts events by time and writes them as one segment, returning the
// location of every resource version (events with a UID that are not core/v1 Events).
func writeSegmentLocated(rootDir, segmentID string, events []models.Event) (segmentBundleMeta, []segmentRecordLocation, error) {
	records := make([]segmentSortableRecord, len(events))
	for i := range events {
		records[i] = segmentSortableRecord{event: events[i], sequence: i}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].event.Timestamp != records[j].event.Timestamp {
			return records[i].event.Timestamp < records[j].event.Timestamp
		}
		return records[i].sequence < records[j].sequence
	})

	builder, err := newSegmentBuilder(rootDir, segmentID)
	if err != nil {
		return segmentBundleMeta{}, nil, err
	}
	for i := range records {
		if err := builder.Append(&records[i].event); err != nil {
			builder.Abort()
			return segmentBundleMeta{}, nil, err
		}
	}
	meta, err := builder.Finish()
	if err != nil {
		return segmentBundleMeta{}, nil, err
	}
	return meta, builder.Locations(), nil
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
		records[i] = segmentSortableRecord{event: events[i], sequence: i}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].event.Timestamp != records[j].event.Timestamp {
			return records[i].event.Timestamp < records[j].event.Timestamp
		}
		return records[i].sequence < records[j].sequence
	})

	builder, err := openSegmentBundleBuilder(segmentDir, segmentID)
	if err != nil {
		return segmentBundleMeta{}, err
	}
	for i := range records {
		if err := builder.Append(&records[i].event); err != nil {
			builder.closeEvents()
			return segmentBundleMeta{}, err
		}
	}
	return builder.finishBundle()
}

// segmentBuilder streams time-ordered events into a new segment bundle. Callers append
// events in non-decreasing timestamp order and then Finish (or Abort) the segment.
type segmentBuilder struct {
	rootDir      string
	segmentsRoot string
	finalDir     string
	tmpDir       string

	segmentID string
	dir       string
	file      *os.File
	writer    *bufio.Writer
	offset    int64
	count     int
	scratch   []byte

	minTimestamp int64
	maxTimestamp int64
	lastOffset   int64

	timeIndexEntries []segmentTimeIndexEntry
	resourceIndex    segmentResourceIndex
	associatedIndex  segmentAssociatedIndex
	dimensionSet     map[segmentDimensionEntry]struct{}
	namespaceKindSet map[string]map[string]struct{}
	locations        []segmentRecordLocation
}

func newSegmentBuilder(rootDir, segmentID string) (*segmentBuilder, error) {
	if rootDir == "" {
		return nil, fmt.Errorf("write segment: root dir is empty")
	}
	if segmentID == "" {
		return nil, fmt.Errorf("write segment: segment id is empty")
	}

	segmentsRoot := filepath.Join(rootDir, segmentsDirName)
	if err := ensureDirWithParentSync(segmentsRoot); err != nil {
		return nil, fmt.Errorf("write segment: ensure segments dir: %w", err)
	}
	tmpRoot := filepath.Join(rootDir, segmentTempDirName)
	if err := ensureDirWithParentSync(tmpRoot); err != nil {
		return nil, fmt.Errorf("write segment: ensure temp dir: %w", err)
	}

	finalDir := filepath.Join(segmentsRoot, segmentID)
	if _, err := os.Stat(finalDir); err == nil {
		return nil, fmt.Errorf("write segment: segment %q already exists", segmentID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("write segment: stat existing segment path: %w", err)
	}

	tmpDir, err := os.MkdirTemp(tmpRoot, segmentID+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("write segment: create temp segment dir: %w", err)
	}

	builder, err := openSegmentBundleBuilder(tmpDir, segmentID)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}
	builder.rootDir = rootDir
	builder.segmentsRoot = segmentsRoot
	builder.finalDir = finalDir
	builder.tmpDir = tmpDir
	return builder, nil
}

func openSegmentBundleBuilder(segmentDir, segmentID string) (*segmentBuilder, error) {
	eventsPath := filepath.Join(segmentDir, segmentEventsFile)
	file, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("write segment bundle: open events file: %w", err)
	}
	return &segmentBuilder{
		segmentID:        segmentID,
		dir:              segmentDir,
		file:             file,
		writer:           bufio.NewWriterSize(file, 256<<10),
		resourceIndex:    segmentResourceIndex{UIDOffsets: make(map[string][]int64)},
		associatedIndex:  segmentAssociatedIndex{InvolvedUIDOffsets: make(map[string][]int64)},
		dimensionSet:     make(map[segmentDimensionEntry]struct{}),
		namespaceKindSet: make(map[string]map[string]struct{}),
	}, nil
}

func (b *segmentBuilder) Count() int {
	return b.count
}

func (b *segmentBuilder) SizeBytes() int64 {
	return b.offset
}

func (b *segmentBuilder) Locations() []segmentRecordLocation {
	return b.locations
}

func (b *segmentBuilder) Append(event *models.Event) error {
	b.scratch = append(b.scratch[:0], 0, 0, 0, 0)
	b.scratch = appendEventRecord(b.scratch, segmentRecordMagic, 0, event, true)
	payloadLen := len(b.scratch) - 4
	if payloadLen > maxSegmentRecordSize {
		return fmt.Errorf("write segment bundle: encode event: payload size %d exceeds max %d", payloadLen, maxSegmentRecordSize)
	}
	binary.BigEndian.PutUint32(b.scratch[:4], uint32(payloadLen))
	return b.appendFrame(event, b.scratch)
}

// appendFrame writes an encoded frame for event and indexes it.
func (b *segmentBuilder) appendFrame(event *models.Event, frame []byte) error {
	if b.count > 0 && event.Timestamp < b.maxTimestamp {
		return fmt.Errorf("write segment bundle: event %q at %d is older than previous record at %d", event.ID, event.Timestamp, b.maxTimestamp)
	}

	recordOffset := b.offset
	if b.count%segmentIndexStride == 0 {
		b.timeIndexEntries = append(b.timeIndexEntries, segmentTimeIndexEntry{Timestamp: event.Timestamp, Offset: recordOffset})
	}
	if _, err := b.writer.Write(frame); err != nil {
		return fmt.Errorf("write segment bundle: write events payload: %w", err)
	}
	b.offset += int64(len(frame))

	if b.count == 0 {
		b.minTimestamp = event.Timestamp
	}
	b.maxTimestamp = event.Timestamp
	b.lastOffset = recordOffset
	b.count++

	resource := &event.Resource
	if resource.UID != "" {
		b.resourceIndex.UIDOffsets[resource.UID] = append(b.resourceIndex.UIDOffsets[resource.UID], recordOffset)
		if resource.Kind != "Event" {
			b.locations = append(b.locations, segmentRecordLocation{
				UID:       resource.UID,
				EventID:   event.ID,
				Timestamp: event.Timestamp,
				Offset:    recordOffset,
				Length:    uint32(len(frame)),
			})
		}
	}
	if resource.Kind == "Event" && resource.InvolvedObjectUID != "" {
		b.associatedIndex.InvolvedUIDOffsets[resource.InvolvedObjectUID] = append(
			b.associatedIndex.InvolvedUIDOffsets[resource.InvolvedObjectUID],
			recordOffset,
		)
	}

	dimension := segmentDimensionEntry{Namespace: resource.Namespace, Kind: resource.Kind}
	if _, ok := b.dimensionSet[dimension]; !ok {
		b.dimensionSet[dimension] = struct{}{}
		kinds, ok := b.namespaceKindSet[dimension.Namespace]
		if !ok {
			kinds = make(map[string]struct{})
			b.namespaceKindSet[dimension.Namespace] = kinds
		}
		kinds[dimension.Kind] = struct{}{}
	}
	return nil
}

func (b *segmentBuilder) closeEvents() {
	if b.file != nil {
		_ = b.file.Close()
		b.file = nil
	}
}

// Abort discards a segment that has not been finished.
func (b *segmentBuilder) Abort() {
	b.closeEvents()
	if b.tmpDir != "" {
		_ = os.RemoveAll(b.tmpDir)
	}
}

// Finish writes the indexes and atomically moves the bundle into the segments directory.
func (b *segmentBuilder) Finish() (segmentBundleMeta, error) {
	meta, err := b.finishBundle()
	if err != nil {
		b.Abort()
		return segmentBundleMeta{}, err
	}
	if err := syncPath(b.tmpDir); err != nil {
		b.Abort()
		return segmentBundleMeta{}, fmt.Errorf("write segment: sync temp segment dir: %w", err)
	}
	if err := os.Rename(b.tmpDir, b.finalDir); err != nil {
		b.Abort()
		return segmentBundleMeta{}, fmt.Errorf("write segment: move segment bundle into place: %w", err)
	}
	b.tmpDir = ""
	if err := syncPath(b.segmentsRoot); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment: sync segments dir: %w", err)
	}
	return meta, nil
}

func (b *segmentBuilder) finishBundle() (segmentBundleMeta, error) {
	if b.count > 0 {
		if len(b.timeIndexEntries) == 0 || b.timeIndexEntries[len(b.timeIndexEntries)-1].Offset != b.lastOffset {
			b.timeIndexEntries = append(b.timeIndexEntries, segmentTimeIndexEntry{Timestamp: b.maxTimestamp, Offset: b.lastOffset})
		}
	}
	if err := b.writer.Flush(); err != nil {
		b.closeEvents()
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: flush events file: %w", err)
	}
	if err := b.file.Sync(); err != nil {
		b.closeEvents()
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: sync events file: %w", err)
	}
	if err := b.file.Close(); err != nil {
		b.file = nil
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: close events file: %w", err)
	}
	b.file = nil

	namespaceKinds := make(map[string][]string, len(b.namespaceKindSet))
	for namespace, kindSet := range b.namespaceKindSet {
		kinds := make([]string, 0, len(kindSet))
		for kind := range kindSet {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		namespaceKinds[namespace] = kinds
	}
	namespaceKinds = normalizeNamespaceKinds(namespaceKinds)

	dimensions := make([]segmentDimensionEntry, 0, len(b.dimensionSet))
	for entry := range b.dimensionSet {
		dimensions = append(dimensions, entry)
	}
	sort.Slice(dimensions, func(i, j int) bool {
		if dimensions[i].Namespace != dimensions[j].Namespace {
			return dimensions[i].Namespace < dimensions[j].Namespace
		}
		return dimensions[i].Kind < dimensions[j].Kind
	})

	meta := segmentBundleMeta{
		ID:             b.segmentID,
		SizeBytes:      b.offset,
		EventCount:     b.count,
		MinTimestamp:   b.minTimestamp,
		MaxTimestamp:   b.maxTimestamp,
		NamespaceKinds: namespaceKinds,
	}

	if err := writeTimeIndex(filepath.Join(b.dir, segmentTimeIndexFile), b.timeIndexEntries); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: time index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(b.dir, segmentUIDIndexFile), b.resourceIndex); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: uid index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(b.dir, segmentAssociatedUIDIndexFile), b.associatedIndex); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: associated uid index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(b.dir, segmentDimIndexFile), segmentDimensionIndex{Entries: dimensions}); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("write segment bundle: dimension index: %w", err)
	}
	if err := writeJSONFile(filepath.Join(b.dir, segmentStatsFile), meta); err != nil {
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
