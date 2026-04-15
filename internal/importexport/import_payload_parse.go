package importexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type parseResult struct {
	Events   []models.Event
	Warnings []string
}

// ParseImportPayload parses a supported import payload and returns normalized events.
func ParseImportPayload(r io.Reader) ([]models.Event, []string, error) {
	result, err := parseImportPayload(r)
	if err != nil {
		return nil, nil, err
	}

	return result.Events, result.Warnings, nil
}

func parseImportPayload(r io.Reader) (*parseResult, error) {
	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read import payload: %w", err)
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	logger := logging.GetLogger("importexport")

	var root map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &root); err == nil {
		if _, hasEvents := root["events"]; hasEvents {
			events, parseErr := parseJSONEvents(bytes.NewReader(trimmed), logger)
			if parseErr != nil {
				return nil, parseErr
			}
			return &parseResult{Events: events}, nil
		}

		var kind string
		if err := json.Unmarshal(root["kind"], &kind); err == nil {
			var apiVersion string
			_ = json.Unmarshal(root["apiVersion"], &apiVersion)
			if (kind == "Event" || kind == "EventList") && isAuditAPIVersion(apiVersion) {
				return parseAuditObjectPayload(trimmed)
			}
		}

		events, parseErr := parseJSONEvents(bytes.NewReader(trimmed), logger)
		if parseErr != nil {
			return nil, parseErr
		}
		return &parseResult{Events: events}, nil
	}

	result, err := parseAuditJSONLPayload(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

func parseImportPayloadInChunks(r io.Reader, chunkSize int, logger *logging.Logger, onChunk ChunkCallback) ([]string, error) {
	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read import payload: %w", err)
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	if looksLikeEventsEnvelope(trimmed) {
		if err := parseJSONEventsInChunks(bytes.NewReader(trimmed), chunkSize, logger, onChunk); err != nil {
			return nil, err
		}
		return nil, nil
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &root); err == nil {
		var kind string
		if err := json.Unmarshal(root["kind"], &kind); err == nil {
			var apiVersion string
			_ = json.Unmarshal(root["apiVersion"], &apiVersion)
			if (kind == "Event" || kind == "EventList") && isAuditAPIVersion(apiVersion) {
				result, parseErr := parseAuditObjectPayload(trimmed)
				if parseErr != nil {
					return nil, parseErr
				}
				if err := emitChunks(result.Events, chunkSize, onChunk); err != nil {
					return nil, err
				}
				return result.Warnings, nil
			}
		}

		if err := parseJSONEventsInChunks(bytes.NewReader(trimmed), chunkSize, logger, onChunk); err != nil {
			return nil, err
		}
		return nil, nil
	}

	result, err := parseAuditJSONLPayload(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	if err := emitChunks(result.Events, chunkSize, onChunk); err != nil {
		return nil, err
	}
	return result.Warnings, nil
}

func looksLikeEventsEnvelope(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))

	firstToken, err := decoder.Token()
	if err != nil {
		return false
	}

	rootDelim, ok := firstToken.(json.Delim)
	if !ok || rootDelim != '{' {
		return false
	}

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return false
		}

		key, ok := keyToken.(string)
		if !ok {
			return false
		}

		if key == "events" {
			return true
		}

		if err := skipJSONValue(decoder); err != nil {
			return false
		}
	}

	return false
}
