package watcher

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// StorageWriter is the interface for writing events to storage
type StorageWriter interface {
	// WriteEvent writes an event to storage
	WriteEvent(event *models.Event) error
}

// EventCaptureHandler captures Kubernetes events and routes them to storage
type EventCaptureHandler struct {
	storage StorageWriter
	logger  *logging.Logger
	pruner  *ManagedFieldsPruner
}

// NewEventCaptureHandler creates a new event capture handler
func NewEventCaptureHandler(storage StorageWriter) *EventCaptureHandler {
	return &EventCaptureHandler{
		storage: storage,
		logger:  logging.GetLogger("event_handler"),
		pruner:  NewManagedFieldsPruner(),
	}
}

// OnAdd handles resource creation events
func (h *EventCaptureHandler) OnAdd(obj runtime.Object) error {
	metadata, err := extractMetadata(obj)
	if err != nil {
		h.logger.Error("Failed to extract metadata from object: %v", err)
		return err
	}

	// Convert object to JSON and prune managedFields
	data, dataSize, err := h.objectToJSON(obj)
	if err != nil {
		h.logger.Error("Failed to convert object to JSON: %v", err)
		return err
	}

	// Create event
	event := &models.Event{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		Type:      models.EventTypeCreate,
		Resource:  metadata,
		Data:      data,
		DataSize:  dataSize,
	}

	// Write to storage
	if err := h.storage.WriteEvent(event); err != nil {
		h.logger.Error("Failed to write CREATE event for %s/%s: %v", metadata.Kind, metadata.Name, err)
		return err
	}

	h.logger.Debug("Captured CREATE event for %s/%s", metadata.Kind, metadata.Name)
	return nil
}

// OnUpdate handles resource update events
func (h *EventCaptureHandler) OnUpdate(oldObj, newObj runtime.Object) error {
	metadata, err := extractMetadata(newObj)
	if err != nil {
		h.logger.Error("Failed to extract metadata from object: %v", err)
		return err
	}

	// Convert object to JSON and prune managedFields
	data, dataSize, err := h.objectToJSON(newObj)
	if err != nil {
		h.logger.Error("Failed to convert object to JSON: %v", err)
		return err
	}

	// Create event
	event := &models.Event{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		Type:      models.EventTypeUpdate,
		Resource:  metadata,
		Data:      data,
		DataSize:  dataSize,
	}

	// Write to storage
	if err := h.storage.WriteEvent(event); err != nil {
		h.logger.Error("Failed to write UPDATE event for %s/%s: %v", metadata.Kind, metadata.Name, err)
		return err
	}

	h.logger.Debug("Captured UPDATE event for %s/%s", metadata.Kind, metadata.Name)
	return nil
}

// OnDelete handles resource deletion events
func (h *EventCaptureHandler) OnDelete(obj runtime.Object) error {
	metadata, err := extractMetadata(obj)
	if err != nil {
		h.logger.Error("Failed to extract metadata from object: %v", err)
		return err
	}

	// For DELETE events, data can be nil or contain the last known state
	data, dataSize, err := h.objectToJSON(obj)
	if err != nil {
		h.logger.Error("Failed to convert object to JSON: %v", err)
		return err
	}

	// Create event
	event := &models.Event{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		Type:      models.EventTypeDelete,
		Resource:  metadata,
		Data:      data,
		DataSize:  dataSize,
	}

	// Write to storage
	if err := h.storage.WriteEvent(event); err != nil {
		h.logger.Error("Failed to write DELETE event for %s/%s: %v", metadata.Kind, metadata.Name, err)
		return err
	}

	h.logger.Debug("Captured DELETE event for %s/%s", metadata.Kind, metadata.Name)
	return nil
}

// objectToJSON converts a Kubernetes object to JSON, pruning managedFields
func (h *EventCaptureHandler) objectToJSON(obj runtime.Object) (json.RawMessage, int32, error) {
	// Marshal to JSON
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal object to JSON: %w", err)
	}

	dataSize := int32(len(jsonData))

	// Prune managedFields to reduce size
	jsonData, err = h.pruner.Prune(jsonData)
	if err != nil {
		h.logger.Warn("Failed to prune managedFields: %v", err)
		// Continue without pruning - don't fail the entire operation
	}

	return json.RawMessage(jsonData), dataSize, nil
}

// extractMetadata extracts resource metadata from a Kubernetes object
func extractMetadata(obj runtime.Object) (models.ResourceMetadata, error) {
	// Get the object's metadata
	accessor, err := apimeta.Accessor(obj)
	if err != nil {
		return models.ResourceMetadata{}, fmt.Errorf("failed to access object metadata: %w", err)
	}

	// Get the object's group version kind
	gvk := obj.GetObjectKind().GroupVersionKind()

	metadata := models.ResourceMetadata{
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		Name:      accessor.GetName(),
		UID:       string(accessor.GetUID()),
		Namespace: accessor.GetNamespace(),
	}

	// Enrich Kubernetes Event objects with involvedObject UID for efficient lookups
	if strings.EqualFold(gvk.Kind, "Event") {
		if involvedUID := extractInvolvedObjectUID(obj); involvedUID != "" {
			metadata.InvolvedObjectUID = involvedUID
		}
	}

	// Validate the metadata
	if err := metadata.Validate(); err != nil {
		return models.ResourceMetadata{}, fmt.Errorf("invalid metadata: %w", err)
	}

	return metadata, nil
}

func extractInvolvedObjectUID(obj runtime.Object) string {
	switch evt := obj.(type) {
	case *corev1.Event:
		return string(evt.InvolvedObject.UID)
	case *unstructured.Unstructured:
		if uid, found, err := unstructured.NestedString(evt.Object, "involvedObject", "uid"); err == nil && found {
			return uid
		}
	default:
		if u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj); err == nil {
			if uid, found, err := unstructured.NestedString(u, "involvedObject", "uid"); err == nil && found {
				return uid
			}
		}
	}
	return ""
}
