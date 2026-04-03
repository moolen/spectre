package api

import (
	"fmt"

	"github.com/moolen/spectre/internal/api/pb"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace"
)

// TimelineService keeps the API package's transport-facing surface while
// delegating timeline orchestration to the app layer.
type TimelineService struct {
	*apptimeline.Service
}

func NewTimelineService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		Service: apptimeline.NewService(queryExecutor, logger, tracer),
	}
}

func NewTimelineServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		Service: apptimeline.NewServiceWithMode(storageExecutor, graphExecutor, toAppQuerySource(querySource), logger, tracer),
	}
}

func toAppQuerySource(querySource TimelineQuerySource) apptimeline.QuerySource {
	switch querySource {
	case TimelineQuerySourceGraph:
		return apptimeline.QuerySourceGraph
	default:
		return apptimeline.QuerySourceStorage
	}
}

// ResourceToProto converts the transport response model into the protobuf shape
// used by the streaming timeline APIs.
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

	pbResource.StatusSegments = make([]*pb.StatusSegment, len(res.StatusSegments))
	for i, seg := range res.StatusSegments {
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
			ResourceData: seg.ResourceData,
		}
	}

	pbResource.Events = make([]*pb.K8SEvent, len(res.Events))
	for i, evt := range res.Events {
		pbResource.Events[i] = &pb.K8SEvent{
			Uid:               evt.ID,
			Type:              evt.Type,
			Reason:            evt.Reason,
			Message:           evt.Message,
			Timestamp:         evt.Timestamp,
			InvolvedObjectUid: res.ID,
		}
	}

	return pbResource
}
