package analysis

import (
	"context"
	"fmt"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

// getChangeEvents retrieves change events for resources within the time window.
// The store preserves the current analyzer semantics: all config changes in-range
// plus a bounded recent context set.
func (a *RootCauseAnalyzer) getChangeEvents(
	ctx context.Context,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (map[string][]ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]ChangeEventInfo), nil
	}

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         lookbackNs,
	}

	a.logger.Debug("getChangeEvents: querying events for %d resources", len(resourceUIDs))
	events, err := a.store.GetChangeEvents(ctx, resourceUIDs, window)
	if err != nil {
		return nil, fmt.Errorf("failed to get change events: %w", err)
	}

	converted := convertStoreChangeEventMap(events)
	for uid, evts := range converted {
		configCount := 0
		for _, event := range evts {
			if event.ConfigChanged {
				configCount++
			}
		}
		a.logger.Debug("getChangeEvents: resource %s has %d events (%d with configChanged=true)", uid, len(evts), configCount)
	}

	a.logger.Debug("getChangeEvents: found events for %d resources", len(converted))
	return converted, nil
}

// getK8sEvents retrieves Kubernetes events (kind: Event) for resources within the time window.
// These are different from ChangeEvents - they represent K8s Events like "FailedScheduling",
// "BackOff", etc. that are emitted by Kubernetes components.
func (a *RootCauseAnalyzer) getK8sEvents(
	ctx context.Context,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (map[string][]K8sEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]K8sEventInfo), nil
	}

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         lookbackNs,
	}

	a.logger.Debug("getK8sEvents: querying events for %d resources", len(resourceUIDs))
	events, err := a.store.GetK8sEvents(ctx, resourceUIDs, window)
	if err != nil {
		return nil, fmt.Errorf("failed to get K8s events: %w", err)
	}

	converted := convertStoreK8sEventMap(events)
	a.logger.Debug("getK8sEvents: found events for %d resources", len(converted))
	return converted, nil
}

// getChangeEventsForRelated retrieves change events for related resources and populates
// the Events field in the RelatedResourceData structures.
//
// This is more efficient than querying events individually for each related resource,
// as it batches all related resource UIDs into a single query.
func (a *RootCauseAnalyzer) getChangeEventsForRelated(
	ctx context.Context,
	relatedByResource map[string][]RelatedResourceData,
	failureTimestamp int64,
	lookbackNs int64,
) error {
	relatedUIDs := []string{}
	uidToParent := make(map[string][]struct {
		parentUID string
		index     int
	})

	for parentUID, relatedList := range relatedByResource {
		for i, rel := range relatedList {
			relatedUIDs = append(relatedUIDs, rel.Resource.UID)
			uidToParent[rel.Resource.UID] = append(uidToParent[rel.Resource.UID], struct {
				parentUID string
				index     int
			}{parentUID, i})
		}
	}

	if len(relatedUIDs) == 0 {
		return nil
	}

	events, err := a.getChangeEvents(ctx, relatedUIDs, failureTimestamp, lookbackNs)
	if err != nil {
		return err
	}

	for uid, eventList := range events {
		for _, parent := range uidToParent[uid] {
			if parent.index < len(relatedByResource[parent.parentUID]) {
				relatedByResource[parent.parentUID][parent.index].Events = eventList
			}
		}
	}

	return nil
}
