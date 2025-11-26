package storage

import (
	"encoding/json"

	"github.com/moritz/rpk/internal/models"
)

// createTestEvent creates a test event with the given parameters
func createTestEvent(name, namespace, kind string, timestamp int64) *models.Event {
	return &models.Event{
		ID:        "test-id-" + name,
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
			UID:       "test-uid-" + name,
		},
		Data: json.RawMessage(`{"test": "data"}`),
	}
}

