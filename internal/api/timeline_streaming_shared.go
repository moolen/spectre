package api

import (
	"fmt"

	"github.com/moolen/spectre/internal/api/pb"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/models"
)

func buildTimelineMetadata(result *models.QueryResult, totalCount int, pagination *models.PaginationResponse) *pb.TimelineMetadata {
	metadata := &pb.TimelineMetadata{
		// Timeline event counts are bounded by database size and query limits.
		// #nosec G115 -- Event counts are bounded by practical query limits
		TotalCount:           int32(totalCount),
		FilesSearched:        result.FilesSearched,
		SegmentsScanned:      result.SegmentsScanned,
		SegmentsSkipped:      result.SegmentsSkipped,
		QueryExecutionTimeMs: int64(result.ExecutionTimeMs),
		NextCursor:           "",
		HasMore:              false,
		PageSize:             0,
	}
	if pagination != nil {
		metadata.NextCursor = pagination.NextCursor
		metadata.HasMore = pagination.HasMore
		metadata.PageSize = int32(pagination.PageSize) // #nosec G115 -- bounded by pagination limits
	}
	return metadata
}

func streamTimelineEntryBatches(
	index *apptimeline.TimelineIndex,
	groups []*GroupedTimelineEntries,
	buildResources func(*apptimeline.TimelineIndex, []*apptimeline.TimelineResourceEntry) []models.Resource,
	toProto func(*models.Resource) *pb.TimelineResource,
	send func(*pb.TimelineChunk) error,
) error {
	for groupIdx, group := range groups {
		isLastGroup := groupIdx == len(groups)-1

		resources := buildResources(index, group.Entries)
		pbResources := make([]*pb.TimelineResource, len(resources))
		for i := range resources {
			pbResources[i] = toProto(&resources[i])
		}

		chunk := &pb.TimelineChunk{
			ChunkType: &pb.TimelineChunk_Batch{
				Batch: &pb.ResourceBatch{
					Kind:         group.Kind,
					Resources:    pbResources,
					IsFinalBatch: isLastGroup,
				},
			},
		}

		if err := send(chunk); err != nil {
			return fmt.Errorf("failed to send batch: %w", err)
		}
	}

	return nil
}
