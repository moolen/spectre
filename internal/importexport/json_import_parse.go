package importexport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/moolen/spectre/internal/importexport/enrichment"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// parseJSONEvents parses a JSON events array from a reader.
func parseJSONEvents(r io.Reader, logger *logging.Logger) ([]models.Event, error) {
	var allEvents []models.Event
	if err := parseJSONEventsInChunks(r, defaultImportChunkSize, logger, func(chunk []models.Event) error {
		allEvents = append(allEvents, chunk...)
		return nil
	}); err != nil {
		return nil, err
	}

	return allEvents, nil
}

func parseJSONEventsInChunks(r io.Reader, chunkSize int, logger *logging.Logger, onChunk ChunkCallback) error {
	logger.Debug("Parsing JSON events")

	decoder := json.NewDecoder(r)
	firstToken, err := decoder.Token()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("empty file or reader")
		}
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	rootDelim, ok := firstToken.(json.Delim)
	if !ok || rootDelim != '{' {
		return fmt.Errorf("failed to parse JSON: expected object with events field")
	}

	eventsFieldFound := false
	rawCount := 0
	validCount := 0
	invalidCount := 0
	chunk := make([]models.Event, 0, chunkSize)
	enricher := enrichment.Default()

	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		validEvents, invalid := validateEvents(chunk, logger)
		invalidCount += invalid
		chunk = chunk[:0]
		if len(validEvents) == 0 {
			return nil
		}
		enricher.Enrich(validEvents, logger)
		validCount += len(validEvents)
		return wrapChunkCallbackError(onChunk(validEvents))
	}

	for decoder.More() {
		keyToken, tokenErr := decoder.Token()
		if tokenErr != nil {
			return fmt.Errorf("failed to parse JSON: %w", tokenErr)
		}

		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("failed to parse JSON: invalid object key type")
		}

		if key != "events" {
			if err := skipJSONValue(decoder); err != nil {
				return fmt.Errorf("failed to parse JSON: %w", err)
			}
			continue
		}

		eventsFieldFound = true

		eventsToken, tokenErr := decoder.Token()
		if tokenErr != nil {
			return fmt.Errorf("failed to parse JSON: %w", tokenErr)
		}
		eventsArray, ok := eventsToken.(json.Delim)
		if !ok || eventsArray != '[' {
			return fmt.Errorf("failed to parse JSON: events field must be an array")
		}

		for decoder.More() {
			var event models.Event
			if decodeErr := decoder.Decode(&event); decodeErr != nil {
				return fmt.Errorf("failed to parse JSON: %w", decodeErr)
			}
			rawCount++
			chunk = append(chunk, event)
			if len(chunk) >= chunkSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}

		endArrayToken, tokenErr := decoder.Token()
		if tokenErr != nil {
			return fmt.Errorf("failed to parse JSON: %w", tokenErr)
		}
		endArray, ok := endArrayToken.(json.Delim)
		if !ok || endArray != ']' {
			return fmt.Errorf("failed to parse JSON: malformed events array")
		}
	}

	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	if !eventsFieldFound || rawCount == 0 {
		return fmt.Errorf("events array is empty")
	}
	if err := flush(); err != nil {
		return err
	}
	if validCount == 0 {
		return fmt.Errorf("all %d events failed validation", invalidCount)
	}
	if invalidCount > 0 {
		logger.WarnWithFields("Some events failed validation",
			logging.Field("valid_count", validCount),
			logging.Field("invalid_count", invalidCount))
	}

	logger.InfoWithFields("JSON parsing completed",
		logging.Field("valid_events", validCount),
		logging.Field("invalid_events", invalidCount))
	return nil
}

func skipJSONValue(decoder *json.Decoder) error {
	var value json.RawMessage
	return decoder.Decode(&value)
}
