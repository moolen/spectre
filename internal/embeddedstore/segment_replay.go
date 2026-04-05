package embeddedstore

import (
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
	exhausted  bool
}

func newSegmentReplayCursor(source replaySegmentReader) *segmentReplayCursor {
	startOffset := int64(0)
	if source.reader != nil {
		if source.startTimestamp == 0 {
			source.startTimestamp = source.reader.meta.MinTimestamp
		}
		if source.endTimestamp == 0 {
			source.endTimestamp = source.reader.meta.MaxTimestamp
		}
		startOffset = source.reader.startOffsetForTime(source.startTimestamp)
	}

	return &segmentReplayCursor{
		source:     source,
		nextOffset: startOffset,
	}
}

func (c *segmentReplayCursor) next(ctx context.Context) (models.Event, bool, error) {
	if c == nil || c.source.reader == nil {
		return models.Event{}, false, fmt.Errorf("replay cursor reader is nil")
	}
	if c.exhausted {
		return models.Event{}, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return models.Event{}, false, err
	}

	if err := c.ensureOpen(); err != nil {
		return models.Event{}, false, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return models.Event{}, false, err
		}

		event, size, err := decodeFramedEvent(c.file)
		if err == nil {
			// Continue below.
		} else if errors.Is(err, io.EOF) {
			c.exhausted = true
			_ = c.close()
			return models.Event{}, false, nil
		} else {
			_ = c.close()
			return models.Event{}, false, fmt.Errorf("decode event: %w", err)
		}

		c.nextOffset += size
		if event.Timestamp < c.source.startTimestamp {
			continue
		}
		if event.Timestamp > c.source.endTimestamp {
			c.exhausted = true
			_ = c.close()
			return models.Event{}, false, nil
		}

		return event, true, nil
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
	return nil
}

func (c *segmentReplayCursor) close() error {
	if c == nil || c.file == nil {
		return nil
	}

	err := c.file.Close()
	c.file = nil
	return err
}

type replayEventHeapItem struct {
	event  models.Event
	cursor *segmentReplayCursor
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

func consumeReplaySegmentReaders(ctx context.Context, sources []replaySegmentReader, sink func(models.Event) error) error {
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
		if sources[i].reader == nil || sources[i].reader.meta.EventCount == 0 {
			continue
		}

		cursor := newSegmentReplayCursor(sources[i])
		cursors = append(cursors, cursor)
		event, ok, err := cursor.next(ctx)
		if err != nil {
			return fmt.Errorf("prime active segment %q: %w", sources[i].segmentID, err)
		}
		if !ok {
			continue
		}

		heap.Push(&queue, &replayEventHeapItem{
			event:  event,
			cursor: cursor,
		})
	}

	for queue.Len() > 0 {
		item := heap.Pop(&queue).(*replayEventHeapItem)
		if err := sink(item.event); err != nil {
			return fmt.Errorf("replay event %q: %w", item.event.ID, err)
		}

		nextEvent, ok, err := item.cursor.next(ctx)
		if err != nil {
			return fmt.Errorf("replay active segment %q: %w", item.cursor.source.segmentID, err)
		}
		if !ok {
			continue
		}

		item.event = nextEvent
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

	if err := consumeReplaySegmentReaders(ctx, sources, func(event models.Event) error {
		if applyProjectionEventUsesDefaultImplementation() {
			return projection.ApplyReplayEvent(event)
		}
		return applyProjectionEvent(projection, event)
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
	if err := consumeReplaySegmentReaders(ctx, sources, func(event models.Event) error {
		projection.appendReplayEvent(event)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("open embedded engine: %w", err)
	}
	projection.finalizeReplayBuild()

	return projection, nil
}
