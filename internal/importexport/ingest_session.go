package importexport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type BatchProcessor interface {
	ProcessBatch(context.Context, []models.Event) error
}

type EventScrubber interface {
	Enabled() bool
	ScrubEventData(kind string, data json.RawMessage) (json.RawMessage, error)
}

type IngestStats struct {
	TotalEvents     int
	TotalChunks     int
	FilesCreated    int
	ProcessDuration time.Duration
}

type IngestSession struct {
	processor BatchProcessor
	logger    *logging.Logger
	scrubber  EventScrubber
	hourSet   map[int64]struct{}
	stats     IngestStats
}

func NewIngestSession(processor BatchProcessor, logger *logging.Logger, scrubber EventScrubber) *IngestSession {
	if logger == nil {
		logger = logging.GetLogger("importexport")
	}

	return &IngestSession{
		processor: processor,
		logger:    logger,
		scrubber:  scrubber,
		hourSet:   make(map[int64]struct{}),
	}
}

func (s *IngestSession) ProcessChunk(ctx context.Context, chunk []models.Event) error {
	s.stats.TotalChunks++
	s.stats.TotalEvents += len(chunk)

	if err := s.scrub(chunk); err != nil {
		return err
	}

	for _, event := range chunk {
		hour := time.Unix(0, event.Timestamp).Truncate(time.Hour).Unix()
		s.hourSet[hour] = struct{}{}
	}
	s.stats.FilesCreated = len(s.hourSet)

	start := time.Now()
	if err := s.processor.ProcessBatch(ctx, chunk); err != nil {
		return err
	}
	s.stats.ProcessDuration += time.Since(start)

	return nil
}

func (s *IngestSession) Stats() IngestStats {
	return s.stats
}

func (s *IngestSession) scrub(events []models.Event) error {
	if s.scrubber == nil || !s.scrubber.Enabled() {
		return nil
	}

	for i := range events {
		if len(events[i].Data) == 0 {
			continue
		}

		scrubbed, err := s.scrubber.ScrubEventData(events[i].Resource.Kind, events[i].Data)
		if err != nil {
			return fmt.Errorf("scrub imported event %s: %w", events[i].ID, err)
		}
		events[i].Data = scrubbed
	}

	return nil
}
