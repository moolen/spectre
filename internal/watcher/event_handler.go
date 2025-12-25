package watcher

import (
	"context"
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

// GraphPipeline is the interface for processing events through the graph pipeline
type GraphPipeline interface {
	ProcessEvent(ctx context.Context, event models.Event) error
}

// TimelineMode specifies where events are written
type TimelineMode string

const (
	TimelineModeStorage TimelineMode = "storage"
	TimelineModeGraph   TimelineMode = "graph"
	TimelineModeBoth    TimelineMode = "both"
)

// EventCaptureHandler captures Kubernetes events and routes them to storage and/or graph
type EventCaptureHandler struct {
	storage       StorageWriter
	graphPipeline GraphPipeline
	mode          TimelineMode
	logger        *logging.Logger
	pruner        *ManagedFieldsPruner
}

// NewEventCaptureHandler creates a new event capture handler (storage-only mode)
func NewEventCaptureHandler(storage StorageWriter) *EventCaptureHandler {
	return &EventCaptureHandler{
		storage: storage,
		mode:    TimelineModeStorage,
		logger:  logging.GetLogger("event_handler"),
		pruner:  NewManagedFieldsPruner(),
	}
}

// NewEventCaptureHandlerWithMode creates an event handler with specified mode
func NewEventCaptureHandlerWithMode(storage StorageWriter, graphPipeline GraphPipeline, mode TimelineMode) *EventCaptureHandler {
	return &EventCaptureHandler{
		storage:       storage,
		graphPipeline: graphPipeline,
		mode:          mode,
		logger:        logging.GetLogger("event_handler"),
		pruner:        NewManagedFieldsPruner(),
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

	// Write based on mode
	return h.writeEvent(event)
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

	// Write based on mode
	return h.writeEvent(event)
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

	// Write based on mode
	return h.writeEvent(event)
}

// writeEvent writes an event based on the configured mode
func (h *EventCaptureHandler) writeEvent(event *models.Event) error {
	ctx := context.Background() // Use background context for event processing

	var storageErr, graphErr error

	// Write to storage if needed
	if h.mode == TimelineModeStorage || h.mode == TimelineModeBoth {
		if h.storage != nil {
			storageErr = h.storage.WriteEvent(event)
			if storageErr != nil {
				h.logger.Error("Failed to write event to storage: %v", storageErr)
			}
		}
	}

	// Write to graph if needed
	if h.mode == TimelineModeGraph || h.mode == TimelineModeBoth {
		if h.graphPipeline != nil {
			graphErr = h.graphPipeline.ProcessEvent(ctx, *event)
			if graphErr != nil {
				h.logger.Error("Failed to write event to graph: %v", graphErr)
			}
		}
	}

	// In "both" mode, succeed if either write succeeds
	if h.mode == TimelineModeBoth {
		if storageErr != nil && graphErr != nil {
			return fmt.Errorf("both writes failed - storage: %v, graph: %v", storageErr, graphErr)
		}
		return nil
	}

	// In single-mode, return the relevant error
	if storageErr != nil {
		return storageErr
	}
	return graphErr
}

// objectToJSON converts a Kubernetes object to JSON, pruning managedFields
func (h *EventCaptureHandler) objectToJSON(obj runtime.Object) (json.RawMessage, int32, error) {
	// Marshal to JSON
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal object to JSON: %w", err)
	}

	dataSize := int32(len(jsonData)) //nolint:gosec // safe conversion: data size is reasonable

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
