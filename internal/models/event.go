package models

import (
	"encoding/json"
	"time"
)

// EventType represents the type of resource change
type EventType string

const (
	// EventTypeCreate represents a resource creation event
	EventTypeCreate EventType = "CREATE"
	// EventTypeUpdate represents a resource update event
	EventTypeUpdate EventType = "UPDATE"
	// EventTypeDelete represents a resource deletion event
	EventTypeDelete EventType = "DELETE"
)

// Event represents a single Kubernetes resource change (CREATE, UPDATE, or DELETE)
type Event struct {
	// ID is a unique identifier for the event (e.g., UUID)
	ID string `json:"id"`

	// Timestamp is when the event was captured (Unix nanoseconds)
	Timestamp int64 `json:"timestamp"`

	// Type indicates the kind of resource change
	Type EventType `json:"type"`

	// Resource contains metadata about the affected resource
	Resource ResourceMetadata `json:"resource"`

	// Data is the full Kubernetes resource object (managedFields removed)
	// Null for DELETE events
	Data json.RawMessage `json:"data,omitempty"`

	// DataSize is the original uncompressed size in bytes
	DataSize int32 `json:"dataSize,omitempty"`

	// CompressedSize is the compressed size in bytes
	CompressedSize int32 `json:"compressedSize,omitempty"`
}

// Validate checks that the event has all required fields and is well-formed
func (e *Event) Validate() error {
	// Validate timestamp
	if e.Timestamp < 0 {
		return NewValidationError("timestamp must be non-negative")
	}

	// Validate type
	if e.Type != EventTypeCreate && e.Type != EventTypeUpdate && e.Type != EventTypeDelete {
		return NewValidationError("type must be one of: CREATE, UPDATE, DELETE")
	}

	// Validate resource metadata
	if err := e.Resource.Validate(); err != nil {
		return err
	}

	// For DELETE events, data should be nil
	// For CREATE/UPDATE events, data should be non-empty JSON
	if e.Type == EventTypeDelete {
		if e.Data != nil && len(e.Data) > 0 {
			// This is acceptable - resource state at deletion time
		}
	} else {
		if e.Data == nil || len(e.Data) == 0 {
			return NewValidationError("data must be present for CREATE and UPDATE events")
		}
	}

	// Validate compression constraint
	if e.CompressedSize > e.DataSize {
		return NewValidationError("compressedSize cannot be greater than dataSize")
	}

	return nil
}

// GetCaptureTime returns the event timestamp as a time.Time
func (e *Event) GetCaptureTime() time.Time {
	return time.Unix(0, e.Timestamp)
}

// IsValid checks if the event is valid
func (e *Event) IsValid() bool {
	return e.Validate() == nil
}
