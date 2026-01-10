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

// GraphPipeline is the interface for processing events through the graph pipeline
type GraphPipeline interface {
	ProcessEvent(ctx context.Context, event models.Event) error
}

// TimelineMode specifies where events are written (graph-only now)
type TimelineMode string

const (
	TimelineModeGraph TimelineMode = "graph"
)

// EventCaptureHandler captures Kubernetes events and routes them to graph
type EventCaptureHandler struct {
	graphPipeline GraphPipeline
	auditLog      AuditLogWriter // Optional audit log
	logger        *logging.Logger
	pruner        *ManagedFieldsPruner
}

// NewEventCaptureHandler creates a new event capture handler (graph-only mode)
func NewEventCaptureHandler(graphPipeline GraphPipeline) *EventCaptureHandler {
	return &EventCaptureHandler{
		graphPipeline: graphPipeline,
		logger:        logging.GetLogger("event_handler"),
		pruner:        NewManagedFieldsPruner(),
	}
}

// SetAuditLog sets the audit log writer for the handler
func (h *EventCaptureHandler) SetAuditLog(writer AuditLogWriter) {
	h.auditLog = writer
}

// NewEventCaptureHandlerWithMode creates an event handler with specified mode (graph-only now)
func NewEventCaptureHandlerWithMode(storage interface{}, graphPipeline GraphPipeline, mode TimelineMode) *EventCaptureHandler {
	// storage parameter is ignored - kept for signature compatibility
	// mode must be TimelineModeGraph
	return &EventCaptureHandler{
		graphPipeline: graphPipeline,
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

// writeEvent writes an event to graph and/or audit log
func (h *EventCaptureHandler) writeEvent(event *models.Event) error {
	ctx := context.Background() // Use background context for event processing

	// Write to audit log FIRST (independent of graph mode)
	if h.auditLog != nil {
		if err := h.auditLog.WriteEvent(event); err != nil {
			h.logger.Warn("Failed to write event to audit log: %v", err)
			// Don't return error - audit log is non-critical
		}
	}

	// Write to graph if available
	if h.graphPipeline != nil {
		if err := h.graphPipeline.ProcessEvent(ctx, *event); err != nil {
			h.logger.Error("Failed to write event to graph: %v", err)
			return err
		}
	} else if h.auditLog == nil {
		// Only error if neither graph nor audit log is configured
		return fmt.Errorf("neither graph pipeline nor audit log is configured")
	}
	// If only audit log is configured (no graph), that's valid - audit-only mode

	return nil
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
