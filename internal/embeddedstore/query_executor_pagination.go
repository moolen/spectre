package embeddedstore

import "github.com/moolen/spectre/internal/models"

func (qe *QueryExecutor) cursorStartIndex(resources []filteredResource, pagination *models.PaginationRequest) int {
	if pagination == nil || pagination.Cursor == "" {
		return 0
	}

	cursor, err := models.DecodeCursor(pagination.Cursor)
	if err != nil || cursor == nil {
		return 0
	}

	for i, resource := range resources {
		if compareCursorKey(resource, cursor) > 0 {
			return i
		}
	}

	return len(resources)
}

func (qe *QueryExecutor) pageBoundsWithCursor(resources []filteredResource, startIdx, pageSize int) (endIdx int, hasMore bool, nextCursor string) {
	if startIdx > len(resources) {
		startIdx = len(resources)
	}

	endIdx = startIdx + pageSize
	if endIdx > len(resources) {
		endIdx = len(resources)
	}

	if endIdx == 0 || endIdx == len(resources) {
		return endIdx, endIdx < len(resources), ""
	}

	lastKey := resources[endIdx-1]
	for endIdx < len(resources) {
		nextKey := resources[endIdx]
		if compareCursorKey(nextKey, &models.ResourceCursor{
			Kind:      lastKey.kind,
			Namespace: lastKey.namespace,
			Name:      lastKey.name,
		}) != 0 {
			break
		}
		endIdx++
	}

	hasMore = endIdx < len(resources)
	if hasMore && endIdx > 0 {
		last := resources[endIdx-1]
		nextCursor = models.NewResourceCursor(last.kind, last.namespace, last.name).Encode()
	}

	return endIdx, hasMore, nextCursor
}

func compareCursorKey(resource filteredResource, cursor *models.ResourceCursor) int {
	if resource.kind != cursor.Kind {
		if resource.kind < cursor.Kind {
			return -1
		}
		return 1
	}
	if resource.namespace != cursor.Namespace {
		if resource.namespace < cursor.Namespace {
			return -1
		}
		return 1
	}
	if resource.name != cursor.Name {
		if resource.name < cursor.Name {
			return -1
		}
		return 1
	}

	return 0
}
