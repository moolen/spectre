package api

import (
	"github.com/moolen/spectre/internal/api/pb"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/models"
)

func parseTimelineProtoRequest(service *apptimeline.Service, req *pb.TimelineRequest) (*models.QueryRequest, *models.PaginationRequest, error) {
	// Build filters - prefer multi-value fields, fallback to single-value for backward compatibility.
	var kinds []string
	if len(req.Kinds) > 0 {
		kinds = req.Kinds
	} else if req.Kind != "" {
		kinds = []string{req.Kind}
	}

	var namespaces []string
	if len(req.Namespaces) > 0 {
		namespaces = req.Namespaces
	} else if req.Namespace != "" {
		namespaces = []string{req.Namespace}
	}

	filters := models.QueryFilters{
		Kinds:      kinds,
		Namespaces: namespaces,
	}

	if err := service.Validator().ValidateFilters(filters); err != nil {
		return nil, nil, err
	}

	queryRequest := &models.QueryRequest{
		StartTimestamp: req.StartTimestamp,
		EndTimestamp:   req.EndTimestamp,
		Filters:        filters,
	}

	if err := queryRequest.Validate(); err != nil {
		return nil, nil, err
	}

	var pagination *models.PaginationRequest
	if req.PageSize > 0 || req.Cursor != "" {
		pagination = &models.PaginationRequest{
			PageSize: int(req.PageSize),
			Cursor:   req.Cursor,
		}
	}

	return queryRequest, pagination, nil
}
