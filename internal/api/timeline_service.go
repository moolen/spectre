package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineService contains shared business logic for timeline operations
// This service is framework-agnostic and used by both gRPC and Connect RPC services
type TimelineService struct {
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	querySource     TimelineQuerySource
	logger          *logging.Logger
	tracer          trace.Tracer
	validator       *Validator
}

// NewTimelineService creates a new timeline service with storage executor only
func NewTimelineService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// NewTimelineServiceWithMode creates a new timeline service with both executors
func NewTimelineServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		querySource:     querySource,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// GetActiveExecutor returns the appropriate query executor based on configuration
func (s *TimelineService) GetActiveExecutor() QueryExecutor {
	switch s.querySource {
	case TimelineQuerySourceGraph:
		if s.graphExecutor != nil {
			return s.graphExecutor
		}
		s.logger.Warn("Graph executor requested but not available, falling back to storage")
		return s.storageExecutor
	case TimelineQuerySourceStorage:
		fallthrough
	default:
		return s.storageExecutor
	}
}

// ResourceToProto converts internal Resource model to protobuf TimelineResource
func (s *TimelineService) ResourceToProto(res *models.Resource) *pb.TimelineResource {
	pbResource := &pb.TimelineResource{
		Id:          res.ID,
		Kind:        res.Kind,
		ApiVersion:  fmt.Sprintf("%s/%s", res.Group, res.Version),
		Namespace:   res.Namespace,
		Name:        res.Name,
		PreExisting: res.PreExisting,
		Labels:      make(map[string]string),
	}

	// Convert status segments
	pbResource.StatusSegments = make([]*pb.StatusSegment, len(res.StatusSegments))
	for i, seg := range res.StatusSegments {
		// Extract reason and determine if status is inferred
		reason, inferred := s.ExtractReasonFromResourceData(seg.ResourceData)
		pbResource.StatusSegments[i] = &pb.StatusSegment{
			Id:           fmt.Sprintf("%s-%d", res.ID, i),
			ResourceId:   res.ID,
			Status:       seg.Status,
			Reason:       reason,
			Message:      seg.Message,
			StartTime:    seg.StartTime,
			EndTime:      seg.EndTime,
			Inferred:     inferred,
			ResourceData: seg.ResourceData, // Full Kubernetes resource JSON
		}
	}

	// Convert K8s events (note: protobuf generated K8SEvent with capital S)
	pbResource.Events = make([]*pb.K8SEvent, len(res.Events))
	for i, evt := range res.Events {
		pbResource.Events[i] = &pb.K8SEvent{
			Uid:               evt.ID,
			Type:              evt.Type,
			Reason:            evt.Reason,
			Message:           evt.Message,
			Timestamp:         evt.Timestamp,
			InvolvedObjectUid: res.ID, // The resource this event belongs to
		}
	}

	return pbResource
}

// ExtractReasonFromResourceData parses the resource JSON and extracts the reason
// from the status conditions. Returns the reason and whether the status was inferred.
func (s *TimelineService) ExtractReasonFromResourceData(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", true // No data means status is inferred
	}

	// Parse the resource data
	var resource map[string]interface{}
	if err := json.Unmarshal(data, &resource); err != nil {
		return "", true // Parse error means status is inferred
	}

	// Extract status.conditions
	status, ok := resource["status"].(map[string]interface{})
	if !ok {
		return "", true // No status means inferred
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok || len(conditions) == 0 {
		return "", true // No conditions means inferred
	}

	// Look for any condition with a reason
	// This unifies the logic from both services:
	// - Connect service looked for first condition with a reason
	// - gRPC service looked specifically for Ready/Healthy conditions
	// We use the more general approach (first condition with reason) as it's more flexible
	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if reason, ok := cond["reason"].(string); ok && reason != "" {
			return reason, false // Found a reason, status is not inferred
		}
	}

	// No condition with reason found - status is inferred
	return "", true
}

// Validator returns the service's validator instance
func (s *TimelineService) Validator() *Validator {
	return s.validator
}

// Logger returns the service's logger instance
func (s *TimelineService) Logger() *logging.Logger {
	return s.logger
}

// Tracer returns the service's tracer instance
func (s *TimelineService) Tracer() trace.Tracer {
	return s.tracer
}

// StorageExecutor returns the storage executor
func (s *TimelineService) StorageExecutor() QueryExecutor {
	return s.storageExecutor
}

// GraphExecutor returns the graph executor
func (s *TimelineService) GraphExecutor() QueryExecutor {
	return s.graphExecutor
}

// QuerySource returns the configured query source
func (s *TimelineService) QuerySource() TimelineQuerySource {
	return s.querySource
}

// ExecuteConcurrentQueries executes resource and Event queries concurrently
func (s *TimelineService) ExecuteConcurrentQueries(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, *models.QueryResult, error) {
	// Create child span for concurrent execution
	ctx, span := s.tracer.Start(ctx, "timeline.executeConcurrentQueries")
	defer span.End()

	// Select which executor to use
	executor := s.GetActiveExecutor()
	if executor == nil {
		return nil, nil, fmt.Errorf("no query executor available")
	}

	span.SetAttributes(attribute.String("query.source", string(s.querySource)))

	var (
		resourceResult *models.QueryResult
		eventResult    *models.QueryResult
		resourceErr    error
		eventErr       error
		wg             sync.WaitGroup
	)

	// Shared cache removed - graph executor doesn't need file coordination
	// Graph queries are handled differently and don't require shared cache

	// Build Event query upfront
	// Use same namespaces filter as the resource query
	eventQuery := &models.QueryRequest{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: query.Filters.GetNamespaces(),
		},
	}

	wg.Add(2)

	// Execute resource query
	go func() {
		defer wg.Done()
		_, resourceSpan := s.tracer.Start(ctx, "timeline.resourceQuery")
		defer resourceSpan.End()

		resourceResult, resourceErr = executor.Execute(ctx, query)
		if resourceErr != nil {
			resourceSpan.RecordError(resourceErr)
			resourceSpan.SetStatus(codes.Error, "Resource query failed")
		}
	}()

	// Execute Event query
	go func() {
		defer wg.Done()
		_, eventSpan := s.tracer.Start(ctx, "timeline.eventQuery")
		defer eventSpan.End()

		eventResult, eventErr = executor.Execute(ctx, eventQuery)
		if eventErr != nil {
			eventSpan.RecordError(eventErr)
			eventSpan.SetStatus(codes.Error, "Event query failed")
			s.logger.Warn("Failed to fetch Kubernetes events for timeline: %v", eventErr)
			// Non-critical: Event query failure shouldn't fail the entire request
		}
	}()

	wg.Wait()

	// Handle errors with priority on resource query (critical)
	if resourceErr != nil {
		return nil, nil, resourceErr
	}

	// If Event query failed, return empty result instead of nil
	if eventErr != nil {
		eventResult = &models.QueryResult{
			Events: []models.Event{},
		}
	}

	span.SetAttributes(
		attribute.Int("resource_count", int(resourceResult.Count)),
		attribute.Int("event_count", int(eventResult.Count)),
	)

	s.logger.Debug("Concurrent queries completed: resources=%d (%dms), events=%d (%dms)",
		resourceResult.Count, resourceResult.ExecutionTimeMs,
		eventResult.Count, eventResult.ExecutionTimeMs)

	return resourceResult, eventResult, nil
}

// BuildTimelineResponse converts query results into a timeline response
func (s *TimelineService) BuildTimelineResponse(queryResult, eventResult *models.QueryResult) *models.SearchResponse {
	if queryResult == nil || len(queryResult.Events) == 0 {
		return &models.SearchResponse{
			Resources:       []models.Resource{},
			Count:           0,
			ExecutionTimeMs: int64(queryResult.ExecutionTimeMs),
		}
	}

	// Group events by resource UID
	eventsByResource := make(map[string][]models.Event)
	queryStartTime := queryResult.Events[0].Timestamp
	queryEndTime := queryResult.Events[0].Timestamp

	for _, event := range queryResult.Events {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}
		eventsByResource[uid] = append(eventsByResource[uid], event)

		// Track actual time range from events
		if event.Timestamp < queryStartTime {
			queryStartTime = event.Timestamp
		}
		if event.Timestamp > queryEndTime {
			queryEndTime = event.Timestamp
		}
	}

	// Build resources with status segments from events
	resourceMap := make(map[string]*models.Resource)

	for uid, events := range eventsByResource {
		if len(events) == 0 {
			continue
		}

		// Sort events by timestamp
		sort.Slice(events, func(i, j int) bool {
			return events[i].Timestamp < events[j].Timestamp
		})

		firstEvent := events[0]
		resourceID := fmt.Sprintf("%s/%s/%s/%s", firstEvent.Resource.Group, firstEvent.Resource.Version, firstEvent.Resource.Kind, uid)

		resource := &models.Resource{
			ID:        resourceID,
			Group:     firstEvent.Resource.Group,
			Version:   firstEvent.Resource.Version,
			Kind:      firstEvent.Resource.Kind,
			Namespace: firstEvent.Resource.Namespace,
			Name:      firstEvent.Resource.Name,
			Events:    []models.K8sEvent{},
		}

		// Build status segments from events
		var segments []models.StatusSegment
		for i, event := range events {
			// Infer status from resource data
			status := analyzer.InferStatusFromResource(event.Resource.Kind, event.Data, string(event.Type))

			// Determine segment end time
			var endTime int64
			if i < len(events)-1 {
				endTime = events[i+1].Timestamp
			} else {
				endTime = queryEndTime
			}

			segment := models.StatusSegment{
				StartTime:    event.Timestamp,
				EndTime:      endTime,
				Status:       status,
				ResourceData: event.Data, // Include full resource data for container issue analysis
			}

			// Extract error message from resource data if available
			if len(event.Data) > 0 {
				errorMessages := analyzer.InferErrorMessages(event.Resource.Kind, event.Data, status)
				if len(errorMessages) > 0 {
					segment.Message = strings.Join(errorMessages, "; ")
				}
			} else {
				// Log warning if data is missing for pod resources (needed for container issue detection)
				if strings.EqualFold(event.Resource.Kind, "Pod") {
					s.logger.Warn("Pod event missing ResourceData in timeline service: %s/%s (event ID: %s, has %d events total)",
						event.Resource.Namespace, event.Resource.Name, event.ID, len(events))
				}
			}

			segments = append(segments, segment)
		}

		resource.StatusSegments = segments
		resourceMap[resourceID] = resource
	}

	// Helper function to safely get string from map
	getString := func(m map[string]interface{}, key, defaultValue string) string {
		if m == nil {
			return defaultValue
		}
		if val, ok := m[key].(string); ok {
			return val
		}
		return defaultValue
	}

	// Attach pre-fetched K8s events
	// Match events to resources by InvolvedObjectUID
	for _, event := range eventResult.Events {
		// Only process Kubernetes Event resources
		if event.Resource.Kind != "Event" {
			continue
		}

		// Match by InvolvedObjectUID
		if event.Resource.InvolvedObjectUID == "" {
			continue
		}

		// Find matching resource by UID
		var targetResource *models.Resource
		for _, resource := range resourceMap {
			// Extract UID from resource ID (format: group/version/kind/uid)
			parts := strings.Split(resource.ID, "/")
			if len(parts) >= 4 {
				resourceUID := parts[3]
				if resourceUID == event.Resource.InvolvedObjectUID {
					targetResource = resource
					break
				}
			}
		}

		if targetResource == nil {
			continue
		}

		// Convert models.Event to models.K8sEvent
		var eventData map[string]interface{}
		if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &eventData); err != nil {
				s.logger.Warn("Failed to parse event data: %v", err)
				continue
			}
		}

		k8sEvent := models.K8sEvent{
			ID:        event.ID,
			Timestamp: event.Timestamp,
			Reason:    getString(eventData, "reason", ""),
			Message:   getString(eventData, "message", ""),
			Type:      getString(eventData, "type", "Normal"),
			Count:     1, // Default count
		}

		// Extract additional fields if present
		if count, ok := eventData["count"].(float64); ok {
			k8sEvent.Count = int32(count)
		}
		if source, ok := eventData["source"].(map[string]interface{}); ok {
			if component, ok := source["component"].(string); ok {
				k8sEvent.Source = component
			}
		}
		if firstTimestamp, ok := eventData["firstTimestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339, firstTimestamp); err == nil {
				k8sEvent.FirstTimestamp = t.UnixNano()
			}
		}
		if lastTimestamp, ok := eventData["lastTimestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339, lastTimestamp); err == nil {
				k8sEvent.LastTimestamp = t.UnixNano()
			}
		}

		targetResource.Events = append(targetResource.Events, k8sEvent)
	}

	resources := make([]models.Resource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		resources = append(resources, *resource)
	}

	return &models.SearchResponse{
		Resources:       resources,
		Count:           len(resources),
		ExecutionTimeMs: int64(queryResult.ExecutionTimeMs),
	}
}
