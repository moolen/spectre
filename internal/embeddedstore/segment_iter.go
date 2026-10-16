package embeddedstore

import (
	"bufio"
	"container/heap"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/moolen/spectre/internal/models"
)

// rawSegmentRecord is one record read for rewriting. For binary records only metadata is
// decoded and frame holds the original bytes so they can be copied without
// recompressing; legacy records are decoded fully and frame is nil.
type rawSegmentRecord struct {
	event  models.Event
	frame  []byte
	offset int64
}

type segmentRecordIterator struct {
	file   *os.File
	reader *bufio.Reader
	offset int64
}

func openSegmentRecordIterator(path string) (*segmentRecordIterator, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &segmentRecordIterator{file: file, reader: bufio.NewReaderSize(file, 256<<10)}, nil
}

func (it *segmentRecordIterator) Close() error {
	if it == nil || it.file == nil {
		return nil
	}
	err := it.file.Close()
	it.file = nil
	return err
}

// next returns io.EOF after the last record.
func (it *segmentRecordIterator) next() (rawSegmentRecord, error) {
	var header [4]byte
	if _, err := io.ReadFull(it.reader, header[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return rawSegmentRecord{}, io.EOF
		}
		return rawSegmentRecord{}, fmt.Errorf("read record header at %d: %w", it.offset, err)
	}
	headerValue := binary.BigEndian.Uint32(header[:])
	compressed := headerValue&segmentRecordCompressedFlag != 0
	payloadLen := headerValue & segmentRecordLengthMask
	if payloadLen > uint32(maxSegmentRecordSize) {
		return rawSegmentRecord{}, fmt.Errorf("record at %d: payload size %d exceeds max %d", it.offset, payloadLen, maxSegmentRecordSize)
	}
	frame := make([]byte, 4+int(payloadLen))
	copy(frame, header[:])
	if _, err := io.ReadFull(it.reader, frame[4:]); err != nil {
		return rawSegmentRecord{}, fmt.Errorf("read record payload at %d: %w", it.offset, err)
	}

	record := rawSegmentRecord{offset: it.offset}
	it.offset += int64(len(frame))
	payload := frame[4:]
	if !compressed && len(payload) > 0 && payload[0] == segmentRecordMagic {
		decoded, err := decodeEventRecordHeader(payload)
		if err != nil {
			return rawSegmentRecord{}, fmt.Errorf("record at %d: %w", record.offset, err)
		}
		record.event = decoded.event
		record.frame = frame
		return record, nil
	}
	event, _, err := decodeFramedPayload(payload, compressed, nil)
	if err != nil {
		return rawSegmentRecord{}, fmt.Errorf("record at %d: %w", record.offset, err)
	}
	record.event = event
	return record, nil
}

// AppendRecord writes a record read by segmentRecordIterator, copying binary frames
// verbatim and re-encoding legacy ones.
func (b *segmentBuilder) AppendRecord(record *rawSegmentRecord) error {
	if record.frame == nil {
		return b.Append(&record.event)
	}
	return b.appendFrame(&record.event, record.frame)
}

type mergeSource struct {
	iterator *segmentRecordIterator
	current  rawSegmentRecord
}

type mergeHeap []*mergeSource

func (h mergeHeap) Len() int { return len(h) }
func (h mergeHeap) Less(i, j int) bool {
	return compareEventOrder(h[i].current.event, h[j].current.event) < 0
}
func (h mergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *mergeHeap) Push(x any)   { *h = append(*h, x.(*mergeSource)) }
func (h *mergeHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}

// mergeSegmentRecords streams the records of several segments in global event order.
func mergeSegmentRecords(paths []string, sink func(*rawSegmentRecord) error) error {
	queue := make(mergeHeap, 0, len(paths))
	defer func() {
		for _, source := range queue {
			_ = source.iterator.Close()
		}
	}()
	for _, path := range paths {
		iterator, err := openSegmentRecordIterator(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		record, err := iterator.next()
		if errors.Is(err, io.EOF) {
			_ = iterator.Close()
			continue
		}
		if err != nil {
			_ = iterator.Close()
			return fmt.Errorf("read %s: %w", path, err)
		}
		queue = append(queue, &mergeSource{iterator: iterator, current: record})
	}
	heap.Init(&queue)

	for queue.Len() > 0 {
		source := queue[0]
		if err := sink(&source.current); err != nil {
			return err
		}
		record, err := source.iterator.next()
		if errors.Is(err, io.EOF) {
			heap.Pop(&queue)
			_ = source.iterator.Close()
			continue
		}
		if err != nil {
			return err
		}
		source.current = record
		heap.Fix(&queue, 0)
	}
	return nil
}

// scanSegmentLocations lists the resource-version records of a segment file.
func scanSegmentLocations(path string) ([]segmentRecordLocation, error) {
	iterator, err := openSegmentRecordIterator(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = iterator.Close()
	}()
	var locations []segmentRecordLocation
	for {
		record, err := iterator.next()
		if errors.Is(err, io.EOF) {
			return locations, nil
		}
		if err != nil {
			return nil, err
		}
		resource := &record.event.Resource
		if resource.UID == "" || resource.Kind == "Event" {
			continue
		}
		length := uint32(iterator.offset - record.offset)
		locations = append(locations, segmentRecordLocation{
			UID:       resource.UID,
			EventID:   record.event.ID,
			Timestamp: record.event.Timestamp,
			Offset:    record.offset,
			Length:    length,
		})
	}
}
