package analysis

import analysisstore "github.com/moolen/spectre/internal/analysis/store"

func convertStoreResourceWithDistanceList(input []analysisstore.ResourceWithDistance) []ResourceWithDistance {
	output := make([]ResourceWithDistance, 0, len(input))
	for _, item := range input {
		output = append(output, ResourceWithDistance{
			Resource: item.Resource,
			Distance: item.Distance,
		})
	}
	return output
}

func convertStoreManagers(input map[string]*analysisstore.ManagerData) map[string]*ManagerData {
	output := make(map[string]*ManagerData, len(input))
	for uid, item := range input {
		if item == nil {
			continue
		}
		output[uid] = &ManagerData{
			Manager:     item.Manager,
			ManagesEdge: item.ManagesEdge,
		}
	}
	return output
}

func convertStoreRelatedResources(input map[string][]analysisstore.RelatedResourceData) map[string][]RelatedResourceData {
	output := make(map[string][]RelatedResourceData, len(input))
	for uid, items := range input {
		output[uid] = make([]RelatedResourceData, 0, len(items))
		for _, item := range items {
			output[uid] = append(output[uid], RelatedResourceData{
				Resource:           item.Resource,
				RelationshipType:   item.RelationshipType,
				Events:             convertStoreChangeEventList(item.Events),
				ReferenceTargetUID: item.ReferenceTargetUID,
			})
		}
	}
	return output
}

func convertStoreChangeEventMap(input map[string][]analysisstore.ChangeEventInfo) map[string][]ChangeEventInfo {
	output := make(map[string][]ChangeEventInfo, len(input))
	for uid, events := range input {
		output[uid] = convertStoreChangeEventList(events)
	}
	return output
}

func convertStoreK8sEventMap(input map[string][]analysisstore.K8sEventInfo) map[string][]K8sEventInfo {
	output := make(map[string][]K8sEventInfo, len(input))
	for uid, events := range input {
		output[uid] = convertStoreK8sEventList(events)
	}
	return output
}

func convertStoreChangeEventList(input []analysisstore.ChangeEventInfo) []ChangeEventInfo {
	output := make([]ChangeEventInfo, 0, len(input))
	for _, event := range input {
		output = append(output, ChangeEventInfo{
			EventID:       event.EventID,
			Timestamp:     event.Timestamp,
			EventType:     event.EventType,
			Status:        event.Status,
			ConfigChanged: event.ConfigChanged,
			StatusChanged: event.StatusChanged,
			Description:   event.Description,
			Significance:  convertStoreEventSignificance(event.Significance),
			Diff:          convertStoreEventDiffs(event.Diff),
			FullSnapshot:  event.FullSnapshot,
			Data:          event.Data,
		})
	}
	return output
}

func convertStoreK8sEventList(input []analysisstore.K8sEventInfo) []K8sEventInfo {
	output := make([]K8sEventInfo, 0, len(input))
	for _, event := range input {
		output = append(output, K8sEventInfo{
			EventID:      event.EventID,
			Timestamp:    event.Timestamp,
			Reason:       event.Reason,
			Message:      event.Message,
			Type:         event.Type,
			Count:        event.Count,
			Source:       event.Source,
			Significance: convertStoreEventSignificance(event.Significance),
		})
	}
	return output
}

func convertStoreEventSignificance(input *analysisstore.EventSignificance) *EventSignificance {
	if input == nil {
		return nil
	}
	return &EventSignificance{
		Score:   input.Score,
		Reasons: append([]string(nil), input.Reasons...),
	}
}

func convertStoreEventDiffs(input []analysisstore.EventDiff) []EventDiff {
	output := make([]EventDiff, 0, len(input))
	for _, diff := range input {
		output = append(output, EventDiff{
			Path:     diff.Path,
			OldValue: diff.OldValue,
			NewValue: diff.NewValue,
			Op:       diff.Op,
		})
	}
	return output
}
