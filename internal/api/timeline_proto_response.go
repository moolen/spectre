package api

import (
	"fmt"

	"github.com/moolen/spectre/internal/api/pb"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/models"
)

// ResourceToProto converts the transport response model into the protobuf shape
// used by the streaming timeline APIs.
func ResourceToProto(service *apptimeline.Service, res *models.Resource) *pb.TimelineResource {
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
		reason, inferred := service.ExtractReasonFromResourceData(seg.ResourceData)
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
