package embeddedstore

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

type segmentReader struct {
	meta              segmentBundleMeta
	eventsPath        string
	timeIndexPath     string
	resourceIndexPath string

	metaLoaded bool
	mu            sync.Mutex
	timeIndex     []segmentTimeIndexEntry
	resourceIndex map[string][]int64
}

func openSegmentReader(rootDir string, meta segmentBundleMeta) (*segmentReader, error) {
	if rootDir == "" {
		return nil, fmt.Errorf("open segment reader: root dir is empty")
	}
	if meta.ID == "" {
		return nil, fmt.Errorf("open segment reader: segment id is empty")
	}

	segmentDir := filepath.Join(rootDir, segmentsDirName, meta.ID)
	meta.NamespaceKinds = normalizeNamespaceKinds(meta.NamespaceKinds)

	return &segmentReader{
		meta:              meta,
		eventsPath:        filepath.Join(segmentDir, segmentEventsFile),
		timeIndexPath:     filepath.Join(segmentDir, segmentTimeIndexFile),
		resourceIndexPath: filepath.Join(segmentDir, segmentUIDIndexFile),
		metaLoaded:        segmentBundleMetaComplete(meta),
	}, nil
}

func (r *segmentReader) MayContain(namespace, kind string) bool {
	if r == nil {
		return false
	}
	if namespace == "" && kind == "" {
		return true
	}

	meta, loaded := r.bundleMeta()
	if !loaded {
		return true
	}
	return meta.MayContain(namespace, kind)
}

func (r *segmentReader) ID() string {
	if r == nil {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.meta.ID
}

func (r *segmentReader) MayContainUID(uid string) bool {
	if r == nil || uid == "" {
		return false
	}
	if err := r.ensureResourceIndexLoaded(); err != nil {
		return true
	}

	offsets := r.resourceIndex[uid]
	return len(offsets) > 0
}

func (r *segmentReader) ScanTimeRange(ctx context.Context, startTimestamp, endTimestamp int64) ([]models.Event, error) {
	if r == nil {
		return nil, fmt.Errorf("scan segment by time: reader is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if startTimestamp > endTimestamp {
		return []models.Event{}, nil
	}
	meta, loaded := r.bundleMeta()
	if loaded {
		if meta.EventCount == 0 {
			return []models.Event{}, nil
		}
		if endTimestamp < meta.MinTimestamp || startTimestamp > meta.MaxTimestamp {
			return []models.Event{}, nil
		}
	}
	if err := r.ensureTimeIndexLoaded(); err != nil {
		return nil, fmt.Errorf("scan segment by time: load time index: %w", err)
	}

	file, err := os.Open(r.eventsPath)
	if err != nil {
		return nil, fmt.Errorf("scan segment by time: open events file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	startOffset := r.startOffsetForTime(startTimestamp)
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("scan segment by time: seek start offset: %w", err)
	}

	events := make([]models.Event, 0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		event, _, err := decodeFramedEvent(file)
		if err == nil {
			// keep processing below
		} else if errors.Is(err, io.EOF) {
			return events, nil
		} else {
			return nil, fmt.Errorf("scan segment by time: decode event: %w", err)
		}

		if event.Timestamp < startTimestamp {
			continue
		}
		if event.Timestamp > endTimestamp {
			return events, nil
		}

		events = append(events, event)
	}
}

func (r *segmentReader) ScanUID(ctx context.Context, uid string) ([]models.Event, error) {
	if r == nil {
		return nil, fmt.Errorf("scan segment by uid: reader is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if uid == "" {
		return []models.Event{}, nil
	}
	if err := r.ensureResourceIndexLoaded(); err != nil {
		return nil, fmt.Errorf("scan segment by uid: load resource index: %w", err)
	}

	offsets := r.resourceIndex[uid]
	if len(offsets) == 0 {
		return []models.Event{}, nil
	}

	file, err := os.Open(r.eventsPath)
	if err != nil {
		return nil, fmt.Errorf("scan segment by uid: open events file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	events := make([]models.Event, 0, len(offsets))
	for i := range offsets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if _, err := file.Seek(offsets[i], io.SeekStart); err != nil {
			return nil, fmt.Errorf("scan segment by uid: seek record %d: %w", i, err)
		}

		event, _, err := decodeFramedEvent(file)
		if err != nil {
			return nil, fmt.Errorf("scan segment by uid: decode record %d: %w", i, err)
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *segmentReader) startOffsetForTime(timestamp int64) int64 {
	if len(r.timeIndex) == 0 {
		return 0
	}

	indexPos := sort.Search(len(r.timeIndex), func(i int) bool {
		return r.timeIndex[i].Timestamp >= timestamp
	})
	if indexPos == 0 {
		return r.timeIndex[0].Offset
	}
	if indexPos == len(r.timeIndex) {
		return r.timeIndex[len(r.timeIndex)-1].Offset
	}

	return r.timeIndex[indexPos-1].Offset
}

func (r *segmentReader) ensureTimeIndexLoaded() error {
	if r == nil {
		return fmt.Errorf("reader is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timeIndex != nil {
		return nil
	}

	timeIndex, err := readTimeIndex(r.timeIndexPath)
	if err != nil {
		return err
	}
	if timeIndex == nil {
		timeIndex = []segmentTimeIndexEntry{}
	}
	r.timeIndex = timeIndex
	return nil
}

func (r *segmentReader) bundleMeta() (segmentBundleMeta, bool) {
	if r == nil {
		return segmentBundleMeta{}, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneSegmentBundleMeta(r.meta), r.metaLoaded
}

func (r *segmentReader) EnsureBundleMeta() (segmentBundleMeta, error) {
	if r == nil {
		return segmentBundleMeta{}, fmt.Errorf("reader is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.metaLoaded {
		return cloneSegmentBundleMeta(r.meta), nil
	}

	meta, err := readSegmentStats(filepath.Join(filepath.Dir(r.eventsPath), segmentStatsFile))
	if err != nil {
		return segmentBundleMeta{}, fmt.Errorf("read stats: %w", err)
	}
	if meta.ID == "" {
		meta.ID = r.meta.ID
	}

	r.meta = meta
	r.metaLoaded = true
	return cloneSegmentBundleMeta(r.meta), nil
}

func (r *segmentReader) ensureResourceIndexLoaded() error {
	if r == nil {
		return fmt.Errorf("reader is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resourceIndex != nil {
		return nil
	}

	resourceIndex, err := readResourceIndex(r.resourceIndexPath)
	if err != nil {
		return err
	}
	r.resourceIndex = resourceIndex.UIDOffsets
	return nil
}

func segmentBundleMetaComplete(meta segmentBundleMeta) bool {
	return meta.EventCount > 0 || meta.MinTimestamp != 0 || meta.MaxTimestamp != 0 || len(meta.NamespaceKinds) > 0
}

func cloneSegmentBundleMeta(meta segmentBundleMeta) segmentBundleMeta {
	return segmentBundleMeta{
		ID:             meta.ID,
		EventCount:     meta.EventCount,
		MinTimestamp:   meta.MinTimestamp,
		MaxTimestamp:   meta.MaxTimestamp,
		NamespaceKinds: normalizeNamespaceKinds(meta.NamespaceKinds),
	}
}

func readSegmentStats(path string) (segmentBundleMeta, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return segmentBundleMeta{}, err
	}

	var meta segmentBundleMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		return segmentBundleMeta{}, fmt.Errorf("decode stats json: %w", err)
	}

	meta.NamespaceKinds = normalizeNamespaceKinds(meta.NamespaceKinds)
	return meta, nil
}

func readTimeIndex(path string) ([]segmentTimeIndexEntry, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(payload)%16 != 0 {
		return nil, fmt.Errorf("invalid time index size %d", len(payload))
	}

	entries := make([]segmentTimeIndexEntry, 0, len(payload)/16)
	for offset := 0; offset < len(payload); offset += 16 {
		entries = append(entries, segmentTimeIndexEntry{
			Timestamp: int64(binary.BigEndian.Uint64(payload[offset : offset+8])),
			Offset:    int64(binary.BigEndian.Uint64(payload[offset+8 : offset+16])),
		})
	}

	return entries, nil
}

func readResourceIndex(path string) (segmentResourceIndex, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return segmentResourceIndex{}, err
	}

	var index segmentResourceIndex
	if err := json.Unmarshal(payload, &index); err != nil {
		return segmentResourceIndex{}, fmt.Errorf("decode resource index json: %w", err)
	}
	if index.UIDOffsets == nil {
		index.UIDOffsets = map[string][]int64{}
	}

	return index, nil
}
