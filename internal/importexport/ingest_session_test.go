package importexport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type fakeBatchProcessor struct {
	batches [][]models.Event
	err     error
}

func (f *fakeBatchProcessor) ProcessBatch(_ context.Context, events []models.Event) error {
	chunk := make([]models.Event, len(events))
	copy(chunk, events)
	f.batches = append(f.batches, chunk)
	return f.err
}

type fakeScrubber struct {
	enabled bool
	err     error
}

func (f fakeScrubber) Enabled() bool {
	return f.enabled
}

func (f fakeScrubber) ScrubEventData(_ string, data json.RawMessage) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append(json.RawMessage("scrubbed:"), data...), nil
}

func TestIngestSession_ProcessChunkTracksCountsAndFilesCreated(t *testing.T) {
	processor := &fakeBatchProcessor{}
	session := NewIngestSession(processor, logging.GetLogger("test"), nil)

	base := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	err := session.ProcessChunk(context.Background(), []models.Event{
		{ID: "a", Timestamp: base.UnixNano()},
		{ID: "b", Timestamp: base.Add(10 * time.Minute).UnixNano()},
		{ID: "c", Timestamp: base.Add(65 * time.Minute).UnixNano()},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	stats := session.Stats()
	if stats.TotalEvents != 3 {
		t.Fatalf("expected 3 events, got %d", stats.TotalEvents)
	}
	if stats.TotalChunks != 1 {
		t.Fatalf("expected 1 chunk, got %d", stats.TotalChunks)
	}
	if stats.FilesCreated != 2 {
		t.Fatalf("expected 2 unique hour buckets, got %d", stats.FilesCreated)
	}
	if len(processor.batches) != 1 || len(processor.batches[0]) != 3 {
		t.Fatalf("unexpected processed batches: %#v", processor.batches)
	}
}

func TestIngestSession_ProcessChunkScrubsEventData(t *testing.T) {
	processor := &fakeBatchProcessor{}
	session := NewIngestSession(processor, logging.GetLogger("test"), fakeScrubber{enabled: true})

	err := session.ProcessChunk(context.Background(), []models.Event{
		{
			ID:   "a",
			Data: []byte(`{"key":"value"}`),
			Resource: models.ResourceMetadata{
				Kind: "Secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := string(processor.batches[0][0].Data); got != `scrubbed:{"key":"value"}` {
		t.Fatalf("unexpected scrubbed data %q", got)
	}
}

func TestIngestSession_ProcessChunkPropagatesScrubError(t *testing.T) {
	processor := &fakeBatchProcessor{}
	session := NewIngestSession(processor, logging.GetLogger("test"), fakeScrubber{enabled: true, err: errors.New("scrub failed")})

	err := session.ProcessChunk(context.Background(), []models.Event{
		{
			ID:   "a",
			Data: []byte(`{"key":"value"}`),
			Resource: models.ResourceMetadata{
				Kind: "Secret",
			},
		},
	})
	if err == nil || err.Error() != "scrub imported event a: scrub failed" {
		t.Fatalf("unexpected error %v", err)
	}
	if len(processor.batches) != 0 {
		t.Fatalf("expected no processed batches, got %#v", processor.batches)
	}
}
