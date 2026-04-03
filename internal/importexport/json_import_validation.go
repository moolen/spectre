package importexport

import (
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// validateEvents validates event data and filters out invalid events.
// Returns: (validEvents, invalidCount)
func validateEvents(events []models.Event, logger *logging.Logger) ([]models.Event, int) {
	validEvents := make([]models.Event, 0, len(events))
	invalidCount := 0

	for i, event := range events {
		if event.ID == "" {
			logger.WarnWithFields("Skipping event with empty ID",
				logging.Field("index", i))
			invalidCount++
			continue
		}
		if event.Timestamp <= 0 {
			logger.WarnWithFields("Skipping event with invalid timestamp",
				logging.Field("event_id", event.ID),
				logging.Field("timestamp", event.Timestamp))
			invalidCount++
			continue
		}
		if event.Type == "" {
			logger.WarnWithFields("Skipping event with empty type",
				logging.Field("event_id", event.ID))
			invalidCount++
			continue
		}
		if event.Resource.Kind == "" {
			logger.WarnWithFields("Skipping event with empty resource kind",
				logging.Field("event_id", event.ID))
			invalidCount++
			continue
		}
		if event.Resource.Name == "" {
			logger.WarnWithFields("Skipping event with empty resource name",
				logging.Field("event_id", event.ID),
				logging.Field("kind", event.Resource.Kind))
			invalidCount++
			continue
		}

		validEvents = append(validEvents, event)
	}

	return validEvents, invalidCount
}
