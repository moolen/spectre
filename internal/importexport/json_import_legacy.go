package importexport

import (
	"io"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// ParseJSONEvents parses a JSON events array from a reader.
//
// Deprecated: Use Import(FromReader(r)) instead.
func ParseJSONEvents(r io.Reader) ([]*models.Event, error) {
	logger := logging.GetLogger("importexport")
	events, err := parseJSONEvents(r, logger)
	if err != nil {
		return nil, err
	}

	eventPtrs := make([]*models.Event, len(events))
	for i := range events {
		eventPtrs[i] = &events[i]
	}
	return eventPtrs, nil
}

// ImportJSONFile reads and parses a single JSON file containing an events array.
//
// Deprecated: Use Import(FromFile(filePath)) instead.
func ImportJSONFile(filePath string) ([]*models.Event, error) {
	logger := logging.GetLogger("importexport")
	events, err := FromFile(filePath).Load(logger)
	if err != nil {
		return nil, err
	}

	eventPtrs := make([]*models.Event, len(events))
	for i := range events {
		eventPtrs[i] = &events[i]
	}
	return eventPtrs, nil
}

// ConvertEventsToValues converts a slice of event pointers to a slice of event values.
//
// Deprecated: The new API returns []models.Event directly. This function is no longer needed.
func ConvertEventsToValues(events []*models.Event) []models.Event {
	eventValues := make([]models.Event, len(events))
	for i, event := range events {
		eventValues[i] = *event
	}
	return eventValues
}

// ImportJSONFileAsValues reads and parses a JSON file and returns events as values.
//
// Deprecated: Use Import(FromFile(filePath)) instead.
func ImportJSONFileAsValues(filePath string) ([]models.Event, error) {
	logger := logging.GetLogger("importexport")
	return FromFile(filePath).Load(logger)
}

// ImportPathAsValues reads and parses events from a file or directory.
//
// Deprecated: Use Import(FromPath(path)) instead.
func ImportPathAsValues(path string) ([]models.Event, error) {
	logger := logging.GetLogger("importexport")
	return FromPath(path).Load(logger)
}
