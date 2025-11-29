package watcher

import (
	"fmt"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// EventValidator validates captured events
type EventValidator struct {
	logger *logging.Logger
}

// NewEventValidator creates a new event validator
func NewEventValidator() *EventValidator {
	return &EventValidator{
		logger: logging.GetLogger("validator"),
	}
}

// ValidateEvent validates a captured event
func (v *EventValidator) ValidateEvent(event *models.Event) error {
	// Check required fields
	if event.ID == "" {
		return fmt.Errorf("event ID must not be empty")
	}

	if event.Timestamp == 0 {
		return fmt.Errorf("event timestamp must be set")
	}

	if event.Type == "" {
		return fmt.Errorf("event type must be set")
	}

	// Validate event type
	if event.Type != models.EventTypeCreate && event.Type != models.EventTypeUpdate && event.Type != models.EventTypeDelete {
		return fmt.Errorf("invalid event type: %s", event.Type)
	}

	// Validate resource metadata
	if err := event.Resource.Validate(); err != nil {
		return fmt.Errorf("invalid resource metadata: %w", err)
	}

	// Validate data for CREATE and UPDATE events
	if event.Type == models.EventTypeCreate || event.Type == models.EventTypeUpdate {
		if len(event.Data) == 0 {
			return fmt.Errorf("data must be present for %s events", event.Type)
		}
	}

	// Validate sizes
	if event.DataSize < 0 {
		return fmt.Errorf("dataSize must be non-negative")
	}

	if event.CompressedSize < 0 {
		return fmt.Errorf("compressedSize must be non-negative")
	}

	if event.CompressedSize > event.DataSize {
		return fmt.Errorf("compressedSize cannot be greater than dataSize")
	}

	return nil
}

// ValidateEventBatch validates a batch of events
func (v *EventValidator) ValidateEventBatch(events []models.Event) []error {
	var errors []error

	for i, event := range events {
		if err := v.ValidateEvent(&event); err != nil {
			errors = append(errors, fmt.Errorf("event %d: %w", i, err))
		}
	}

	return errors
}

// LogValidationError logs a validation error with context
func (v *EventValidator) LogValidationError(event *models.Event, err error) {
	v.logger.Warn("Event validation failed for %s/%s (type=%s): %v",
		event.Resource.Kind, event.Resource.Name, event.Type, err)
}

// GetValidationSummary returns a summary of validation for a batch
func (v *EventValidator) GetValidationSummary(events []models.Event) map[string]int {
	summary := map[string]int{
		"total":   len(events),
		"valid":   0,
		"invalid": 0,
		"by_type": 0,
	}

	for _, event := range events {
		if v.ValidateEvent(&event) == nil {
			summary["valid"]++
		} else {
			summary["invalid"]++
		}
	}

	return summary
}
