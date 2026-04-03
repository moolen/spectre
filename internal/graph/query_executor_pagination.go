package graph

import (
	"sort"

	"github.com/moolen/spectre/internal/models"
)

type resourceInfo struct {
	uid    string
	kind   string
	ns     string
	name   string
	events []models.Event
}

func (qe *QueryExecutor) paginateTimelineEvents(allEvents []models.Event, pageSize int) ([]models.Event, string, bool) {
	eventsByResource := qe.groupEventsByResource(allEvents)
	resources := resourcesFromEvents(eventsByResource)

	qe.logger.Debug("Resource-based pagination: %d events grouped into %d unique resources (pageSize=%d)",
		len(allEvents), len(resources), pageSize)

	sort.Slice(resources, func(i, j int) bool {
		if resources[i].kind != resources[j].kind {
			return resources[i].kind < resources[j].kind
		}
		if resources[i].ns != resources[j].ns {
			return resources[i].ns < resources[j].ns
		}
		return resources[i].name < resources[j].name
	})

	limitedEvents := make([]models.Event, 0, len(allEvents))
	lastResourceIdx := -1
	seenResources := 0
	for i := 0; i < len(resources) && seenResources < pageSize; i++ {
		limitedEvents = append(limitedEvents, resources[i].events...)
		seenResources++
		lastResourceIdx = i
	}

	hasMore := len(resources) > pageSize
	nextCursor := ""
	if hasMore && lastResourceIdx >= 0 && lastResourceIdx < len(resources) {
		lastRes := resources[lastResourceIdx]
		nextCursor = models.NewResourceCursor(lastRes.kind, lastRes.ns, lastRes.name).Encode()
		qe.logger.Debug("Generated nextCursor from last resource: kind=%s, ns=%s, name=%s",
			lastRes.kind, lastRes.ns, lastRes.name)
	}

	qe.logger.Debug("Resource pagination result: %d events from %d resources, hasMore=%v, nextCursor=%q",
		len(limitedEvents), seenResources, hasMore, nextCursor)

	return limitedEvents, nextCursor, hasMore
}

func (qe *QueryExecutor) groupEventsByResource(allEvents []models.Event) map[string][]models.Event {
	eventsByResource := make(map[string][]models.Event)
	for _, event := range allEvents {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}
		eventsByResource[uid] = append(eventsByResource[uid], event)
	}
	return eventsByResource
}

func resourcesFromEvents(eventsByResource map[string][]models.Event) []resourceInfo {
	resources := make([]resourceInfo, 0, len(eventsByResource))
	for uid, events := range eventsByResource {
		if len(events) == 0 {
			continue
		}
		firstEvent := events[0]
		resources = append(resources, resourceInfo{
			uid:    uid,
			kind:   firstEvent.Resource.Kind,
			ns:     firstEvent.Resource.Namespace,
			name:   firstEvent.Resource.Name,
			events: events,
		})
	}
	return resources
}
