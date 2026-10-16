package embeddedstore

import (
	"bufio"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/moolen/spectre/internal/models"
)

type replaySegmentReader struct {
	segmentID      string
	reader         *segmentReader
	startTimestamp int64
	endTimestamp   int64
}

type segmentReplayCursor struct {
	source     replaySegmentReader
	nextOffset int64
	file       *os.File
	reader     *bufio.Reader
	exhausted  bool
}

func newSegmentReplayCursor(source replaySegmentReader) *segmentReplayCursor {
	startOffset := int64(0)
	if source.reader != nil {
		if source.startTimestamp == 0 || source.endTimestamp == 0 {
			if meta, err := source.reader.EnsureBundleMeta(); err == nil {
				if source.startTimestamp == 0 {
					source.startTimestamp = meta.MinTimestamp
				}
				if source.endTimestamp == 0 {
					source.endTimestamp = meta.MaxTimestamp
				}
			}
		}
		startOffset = source.reader.startOffsetForTime(source.startTimestamp)
	}

	return &segmentReplayCursor{
		source:     source,
		nextOffset: startOffset,
	}
}

// replayLocation is where a replayed event's record lives, so the projection can
// reference the payload instead of keeping it.
type replayLocation struct {
	segmentID string
	offset    int64
	length    uint32
}

func (c *segmentReplayCursor) next(ctx context.Context) (models.Event, bool, error) {
	event, _, ok, err := c.nextLocated(ctx)
	return event, ok, err
}

func (c *segmentReplayCursor) nextLocated(ctx context.Context) (models.Event, replayLocation, bool, error) {
	if c == nil || c.source.reader == nil {
		return models.Event{}, replayLocation{}, false, fmt.Errorf("replay cursor reader is nil")
	}
	if c.exhausted {
		return models.Event{}, replayLocation{}, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return models.Event{}, replayLocation{}, false, err
	}

	if err := c.ensureOpen(); err != nil {
		return models.Event{}, replayLocation{}, false, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return models.Event{}, replayLocation{}, false, err
		}

		event, size, err := decodeFramedEvent(c.reader)
		if err == nil {
			// Continue below.
		} else if errors.Is(err, io.EOF) {
			c.exhausted = true
			_ = c.close()
			return models.Event{}, replayLocation{}, false, nil
		} else {
			_ = c.close()
			return models.Event{}, replayLocation{}, false, fmt.Errorf("decode event: %w", err)
		}

		location := replayLocation{segmentID: c.source.segmentID, offset: c.nextOffset, length: uint32(size)}
		c.nextOffset += size
		if event.Timestamp < c.source.startTimestamp {
			continue
		}
		if event.Timestamp > c.source.endTimestamp {
			c.exhausted = true
			_ = c.close()
			return models.Event{}, replayLocation{}, false, nil
		}

		return event, location, true, nil
	}
}

func (c *segmentReplayCursor) ensureOpen() error {
	if c == nil || c.source.reader == nil {
		return fmt.Errorf("replay cursor reader is nil")
	}
	if c.file != nil {
		return nil
	}

	file, err := os.Open(c.source.reader.eventsPath)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	if _, err := file.Seek(c.nextOffset, io.SeekStart); err != nil {
		_ = file.Close()
		return fmt.Errorf("seek next offset: %w", err)
	}

	c.file = file
	c.reader = bufio.NewReaderSize(file, 256<<10)
	return nil
}

func (c *segmentReplayCursor) close() error {
	if c == nil || c.file == nil {
		return nil
	}

	err := c.file.Close()
	c.file = nil
	c.reader = nil
	return err
}

type replayEventHeapItem struct {
	event    models.Event
	location replayLocation
	cursor   *segmentReplayCursor
}

type replayEventHeap []*replayEventHeapItem

func (h replayEventHeap) Len() int {
	return len(h)
}

func (h replayEventHeap) Less(i, j int) bool {
	return compareEventOrder(h[i].event, h[j].event) < 0
}

func (h replayEventHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *replayEventHeap) Push(x any) {
	*h = append(*h, x.(*replayEventHeapItem))
}

func (h *replayEventHeap) Pop() any {
	old := *h
	last := len(old) - 1
	item := old[last]
	*h = old[:last]
	return item
}

func consumeReplaySegmentReaders(ctx context.Context, sources []replaySegmentReader, sink func(models.Event, replayLocation) error) error {
	if sink == nil {
		return fmt.Errorf("replay sink is nil")
	}
	queue := make(replayEventHeap, 0, len(sources))
	cursors := make([]*segmentReplayCursor, 0, len(sources))
	defer func() {
		for i := range cursors {
			_ = cursors[i].close()
		}
	}()
	for i := range sources {
		if sources[i].reader == nil {
			continue
		}
		meta, err := sources[i].reader.EnsureBundleMeta()
		if err != nil {
			return fmt.Errorf("load active segment %q metadata: %w", sources[i].segmentID, err)
		}
		if meta.EventCount == 0 {
			continue
		}
		if sources[i].startTimestamp == 0 {
			sources[i].startTimestamp = meta.MinTimestamp
		}
		if sources[i].endTimestamp == 0 {
			sources[i].endTimestamp = meta.MaxTimestamp
		}

		cursor := newSegmentReplayCursor(sources[i])
		cursors = append(cursors, cursor)
		event, location, ok, err := cursor.nextLocated(ctx)
		if err != nil {
			return fmt.Errorf("prime active segment %q: %w", sources[i].segmentID, err)
		}
		if !ok {
			continue
		}

		heap.Push(&queue, &replayEventHeapItem{
			event:    event,
			location: location,
			cursor:   cursor,
		})
	}

	for queue.Len() > 0 {
		item := heap.Pop(&queue).(*replayEventHeapItem)
		if err := sink(item.event, item.location); err != nil {
			return fmt.Errorf("replay event %q: %w", item.event.ID, err)
		}

		nextEvent, nextLocation, ok, err := item.cursor.nextLocated(ctx)
		if err != nil {
			return fmt.Errorf("replay active segment %q: %w", item.cursor.source.segmentID, err)
		}
		if !ok {
			continue
		}

		item.event = nextEvent
		item.location = nextLocation
		heap.Push(&queue, item)
	}

	return nil
}

func replaySegmentReaders(ctx context.Context, projection *Projection, sources []replaySegmentReader) error {
	if projection == nil {
		return fmt.Errorf("replay projection is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := consumeReplaySegmentReaders(ctx, sources, func(event models.Event, location replayLocation) error {
		var err error
		if applyProjectionEventUsesDefaultImplementation() {
			err = projection.ApplyReplayEvent(event)
		} else {
			err = applyProjectionEvent(projection, event)
		}
		if err != nil {
			return err
		}
		projection.attachReplayLocation(event, location)
		return nil
	}); err != nil {
		return fmt.Errorf("open embedded engine: %w", err)
	}

	return nil
}

func buildProjectionFromReplayReaders(ctx context.Context, sources []replaySegmentReader) (*Projection, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	projection := NewProjection()
	if err := consumeReplaySegmentReaders(ctx, sources, func(event models.Event, location replayLocation) error {
		projection.appendReplayEvent(event)
		projection.attachReplayLocation(event, location)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("open embedded engine: %w", err)
	}
	projection.finalizeReplayBuild()

	return projection, nil
}
