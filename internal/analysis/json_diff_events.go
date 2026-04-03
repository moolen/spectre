package analysis

// ConvertEventsToDiffFormat converts a slice of legacy events into diff-aware events.
func ConvertEventsToDiffFormat(events []ChangeEventInfo, filterNoisy bool) []ChangeEventInfo {
	if len(events) == 0 {
		return events
	}

	reversed := make([]ChangeEventInfo, len(events))
	for i, event := range events {
		reversed[len(events)-1-i] = event
	}

	result := make([]ChangeEventInfo, len(events))
	var prevData []byte

	for i, event := range reversed {
		result[i] = event

		if i == 0 {
			snapshot, err := ParseJSONToMap(event.Data)
			if err == nil && snapshot != nil {
				result[i].FullSnapshot = snapshot
			}
			prevData = event.Data
			continue
		}

		diffs, err := ComputeJSONDiff(prevData, event.Data)
		if err == nil {
			if filterNoisy {
				diffs = FilterNoisyPaths(diffs)
			}
			result[i].Diff = diffs
		}
		prevData = event.Data
	}

	final := make([]ChangeEventInfo, len(events))
	for i := range result {
		final[len(result)-1-i] = result[i]
	}

	return final
}

// ConvertSingleEventToDiff converts a single event to diff format given previous data.
func ConvertSingleEventToDiff(event *ChangeEventInfo, prevData []byte, filterNoisy bool) {
	if event == nil {
		return
	}

	if len(prevData) == 0 {
		snapshot, err := ParseJSONToMap(event.Data)
		if err == nil && snapshot != nil {
			event.FullSnapshot = snapshot
		}
	} else {
		diffs, err := ComputeJSONDiff(prevData, event.Data)
		if err == nil {
			if filterNoisy {
				diffs = FilterNoisyPaths(diffs)
			}
			event.Diff = diffs
		}
	}

	event.Data = nil
}
