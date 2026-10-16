package embeddedstore

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

// lazyEvent is an event whose payload may still be on disk. Scans that only need
// metadata to filter, group and paginate keep payloads unread and hydrate the events
// that end up in the response.
type lazyEvent struct {
	event  models.Event
	path   string
	offset int64
	length uint32
	loaded bool
}

func loadedLazyEvent(event models.Event) lazyEvent {
	return lazyEvent{event: event, loaded: true}
}

// scanTimeRangeLazy returns the records in [start, end] accepted by keep. Without
// withData only metadata is decoded; legacy JSON records are always decoded fully.
// scanTimeRangeLazy returns the segment's records in [startTimestamp, endTimestamp] that
// pass prefilter (checked on the raw header) and keep. withData decodes payloads now;
// otherwise they are read later by hydrateLazyEvents.
func (r *segmentReader) scanTimeRangeLazy(ctx context.Context, startTimestamp, endTimestamp int64, prefilter *recordPrefilter, keep func(*models.Event) bool, withData bool) ([]lazyEvent, error) {
	if r == nil {
		return nil, fmt.Errorf("scan segment by time: reader is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if startTimestamp > endTimestamp {
		return nil, nil
	}
	if meta, loaded := r.bundleMeta(); loaded {
		if meta.EventCount == 0 || endTimestamp < meta.MinTimestamp || startTimestamp > meta.MaxTimestamp {
			return nil, nil
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
	offset := r.startOffsetForTime(startTimestamp)
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("scan segment by time: seek start offset: %w", err)
	}
	reader := bufio.NewReaderSize(file, 256<<10)

	var result []lazyEvent
	var header [4]byte
	var payload []byte
	for records := 0; ; records++ {
		if records%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return nil, fmt.Errorf("scan segment by time: read header at %d: %w", offset, err)
		}
		headerValue := binary.BigEndian.Uint32(header[:])
		compressed := headerValue&segmentRecordCompressedFlag != 0
		payloadLen := int(headerValue & segmentRecordLengthMask)
		if payloadLen > maxSegmentRecordSize {
			return nil, fmt.Errorf("scan segment by time: record at %d exceeds max size", offset)
		}
		if cap(payload) < payloadLen {
			payload = make([]byte, payloadLen)
		}
		payload = payload[:payloadLen]
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, fmt.Errorf("scan segment by time: read record at %d: %w", offset, err)
		}
		recordOffset := offset
		offset += int64(4 + payloadLen)

		if !compressed && payloadLen > 0 && payload[0] == segmentRecordMagic {
			if key, ok := peekEventRecordKey(payload); ok {
				if key.timestamp < startTimestamp {
					continue
				}
				if key.timestamp > endTimestamp {
					return result, nil
				}
				if !prefilter.matches(&key) {
					continue
				}
			}
			record, err := decodeEventRecordHeader(payload)
			if err != nil {
				return nil, fmt.Errorf("scan segment by time: record at %d: %w", recordOffset, err)
			}
			if record.event.Timestamp < startTimestamp {
				continue
			}
			if record.event.Timestamp > endTimestamp {
				return result, nil
			}
			if keep != nil && !keep(&record.event) {
				continue
			}
			if withData {
				if err := record.decodeData(); err != nil {
					return nil, fmt.Errorf("scan segment by time: record at %d: %w", recordOffset, err)
				}
				result = append(result, loadedLazyEvent(record.event))
				continue
			}
			result = append(result, lazyEvent{
				event:  record.event,
				path:   r.eventsPath,
				offset: recordOffset,
				length: uint32(4 + payloadLen),
				loaded: record.dataEncoding == eventDataNone,
			})
			continue
		}

		event, _, err := decodeFramedPayload(payload, compressed, nil)
		if err != nil {
			return nil, fmt.Errorf("scan segment by time: record at %d: %w", recordOffset, err)
		}
		if event.Timestamp < startTimestamp {
			continue
		}
		if event.Timestamp > endTimestamp {
			return result, nil
		}
		if keep != nil && !keep(&event) {
			continue
		}
		result = append(result, loadedLazyEvent(event))
	}
}

// hydrateLazyEvents reads the payloads of the given events, opening each segment once.
func hydrateLazyEvents(events []*lazyEvent) error {
	byPath := make(map[string][]*lazyEvent)
	for _, event := range events {
		if event.loaded {
			continue
		}
		byPath[event.path] = append(byPath[event.path], event)
	}
	for path, pending := range byPath {
		if err := hydrateFromFile(path, pending); err != nil {
			return err
		}
	}
	return nil
}

func hydrateFromFile(path string, events []*lazyEvent) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("hydrate events: open %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()
	sort.Slice(events, func(i, j int) bool { return events[i].offset < events[j].offset })
	for _, event := range events {
		buf := make([]byte, event.length)
		if _, err := file.ReadAt(buf, event.offset); err != nil {
			return fmt.Errorf("hydrate events: read %s at %d: %w", path, event.offset, err)
		}
		decoded, _, err := decodeFramedPayload(buf[4:], binary.BigEndian.Uint32(buf[:4])&segmentRecordCompressedFlag != 0, nil)
		if err != nil {
			return fmt.Errorf("hydrate events: decode %s at %d: %w", path, event.offset, err)
		}
		event.event.Data = decoded.Data
		event.loaded = true
	}
	return nil
}

// exportTimeRangeLazy is exportTimeRange without reading payloads or ordering the result.
func (p *QueryPlanner) exportTimeRangeLazy(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
) ([]lazyEvent, queryPlanStats, error) {
	startTimeNs = p.clampToRetention(startTimeNs)
	if p == nil || endTimeNs < startTimeNs {
		return nil, queryPlanStats{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, queryPlanStats{}, err
	}

	stats := queryPlanStats{}
	keep := func(event *models.Event) bool { return filters.Matches(event.Resource) }
	prefilter := newRecordPrefilter(filters)
	var exported []lazyEvent
	relevant := p.relevantExportSegments(filters, startTimeNs, endTimeNs)
	stats.relevantSegments = len(relevant)
	if len(relevant) > 0 {
		stats.coldUsed = true
		stats.scannedSegments += len(relevant)
	}
	perSegment, err := scanSegmentsParallel(ctx, relevant, func(reader *segmentReader) ([]lazyEvent, error) {
		return reader.scanTimeRangeLazy(ctx, startTimeNs, endTimeNs, prefilter, keep, false)
	})
	if err != nil {
		return nil, stats, err
	}
	for _, events := range perSegment {
		exported = append(exported, events...)
	}

	if p.hot != nil {
		hotEvents := p.hot.ScanTimeRangeMatching(startTimeNs, endTimeNs, func(event *models.Event) bool {
			return filters.Matches(event.Resource)
		})
		if len(hotEvents) > 0 {
			stats.hotUsed = true
			stats.hotScans++
		}
		for _, event := range hotEvents {
			exported = append(exported, loadedLazyEvent(event))
		}
	}
	if len(exported) == 0 {
		return nil, stats, nil
	}

	// Unordered: the caller groups and orders a page at a time, which is far cheaper than
	// sorting every event of a long window.
	return dedupeLazyEventsByID(exported), stats, nil
}

// scanSegmentsParallel runs scan over the readers on up to GOMAXPROCS goroutines and
// returns the results in reader order. Long windows span dozens of segments, and each
// scan is independent.
func scanSegmentsParallel(ctx context.Context, readers []*segmentReader, scan func(*segmentReader) ([]lazyEvent, error)) ([][]lazyEvent, error) {
	results := make([][]lazyEvent, len(readers))
	errs := make([]error, len(readers))
	workers := min(len(readers), runtime.GOMAXPROCS(0))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i], errs[i] = scan(readers[i])
			}
		}()
	}
	for i := range readers {
		if ctx.Err() != nil {
			break
		}
		next <- i
	}
	close(next)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("scan segment %q by time: %w", readers[i].ID(), err)
		}
	}
	return results, nil
}

// dedupeLazyEventsByID keeps one event per ID, preferring an already loaded copy. The
// same event can be seen twice while a flush moves it from the hot store to a segment.
func dedupeLazyEventsByID(events []lazyEvent) []lazyEvent {
	if len(events) <= 1 {
		return events
	}
	index := make(map[string]int, len(events))
	deduped := events[:0:0]
	for i := range events {
		position, seen := index[events[i].event.ID]
		if !seen {
			index[events[i].event.ID] = len(deduped)
			deduped = append(deduped, events[i])
			continue
		}
		existing := &deduped[position]
		if compareDuplicateLazyPreference(events[i], *existing) < 0 {
			*existing = events[i]
		}
	}
	return deduped
}

func compareDuplicateLazyPreference(left, right lazyEvent) int {
	if cmp := compareEventOrder(left.event, right.event); cmp != 0 {
		return cmp
	}
	if left.event.PreExisting != right.event.PreExisting {
		if !left.event.PreExisting {
			return -1
		}
		return 1
	}
	if cmp := compareResourceMetadata(left.event.Resource, right.event.Resource); cmp != 0 {
		return cmp
	}
	if left.loaded != right.loaded {
		if left.loaded {
			return -1
		}
		return 1
	}
	return 0
}
