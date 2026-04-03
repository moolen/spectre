package anomaly

import (
	"encoding/json"

	"github.com/moolen/spectre/internal/analysis"
)

func eventSnapshotData(event analysis.ChangeEventInfo) map[string]interface{} {
	if event.FullSnapshot != nil {
		return event.FullSnapshot
	}

	if len(event.Data) == 0 {
		return nil
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(event.Data, &resourceData); err != nil {
		return nil
	}

	return resourceData
}
