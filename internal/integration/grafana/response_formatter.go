package grafana

import (
	"encoding/json"
	"time"
)

// DashboardQueryResult represents the result of executing queries for a dashboard.
// Contains successful panel results and any errors for failed panels.
type DashboardQueryResult struct {
	DashboardUID   string        `json:"dashboard_uid"`
	DashboardTitle string        `json:"dashboard_title"`
	Panels         []PanelResult `json:"panels"`           // Successful panels only
	Errors         []PanelError  `json:"errors,omitempty"` // Failed panels
	TimeRange      string        `json:"time_range"`
}

// PanelResult represents the result of executing queries for a single panel.
type PanelResult struct {
	PanelID    int            `json:"panel_id"`
	PanelTitle string         `json:"panel_title"`
	Query      string         `json:"query,omitempty"` // PromQL, only on empty results
	Metrics    []MetricSeries `json:"metrics"`
}

// PanelError represents a failed panel query.
type PanelError struct {
	PanelID    int    `json:"panel_id"`
	PanelTitle string `json:"panel_title"`
	Query      string `json:"query"`
	Error      string `json:"error"`
}

// MetricSeries represents a time series with labels and data points.
type MetricSeries struct {
	Labels map[string]string `json:"labels"`
	Unit   string            `json:"unit,omitempty"`
	Values []DataPoint       `json:"values"`
}

// DataPoint represents a single timestamp-value pair.
type DataPoint struct {
	Timestamp string  `json:"timestamp"` // ISO8601 format
	Value     float64 `json:"value"`
}

// formatTimeSeriesResponse converts a Grafana QueryResponse into a PanelResult.
// panelID: the panel's ID
// panelTitle: the panel's title
// query: the PromQL query that was executed
// response: the QueryResponse from Grafana
// Returns a PanelResult with metrics extracted from the response.
// If the response has no data, the Query field will be populated for debugging.
func formatTimeSeriesResponse(panelID int, panelTitle string, query string, response *QueryResponse) *PanelResult {
	result := &PanelResult{
		PanelID:    panelID,
		PanelTitle: panelTitle,
		Metrics:    make([]MetricSeries, 0),
	}

	// Check if we have results
	if response == nil || len(response.Results) == 0 {
		result.Query = query // Include query for empty results
		return result
	}

	// Extract metrics from all result frames
	for _, queryResult := range response.Results {
		for _, frame := range queryResult.Frames {
			series := extractMetricSeries(frame)
			if series != nil && len(series.Values) > 0 {
				result.Metrics = append(result.Metrics, *series)
			}
		}
	}

	// Include query if no metrics extracted (empty result)
	if len(result.Metrics) == 0 {
		result.Query = query
	}

	return result
}

// extractMetricSeries extracts a MetricSeries from a single DataFrame.
// Returns nil if the frame has no data.
func extractMetricSeries(frame DataFrame) *MetricSeries {
	// Need at least 2 fields (timestamp and value)
	if len(frame.Schema.Fields) < 2 {
		return nil
	}

	// Need at least some values
	if len(frame.Data.Values) < 2 {
		return nil
	}

	timestamps := frame.Data.Values[0]
	values := frame.Data.Values[1]

	if len(timestamps) == 0 || len(values) == 0 {
		return nil
	}

	series := &MetricSeries{
		Labels: make(map[string]string),
		Values: make([]DataPoint, 0, len(timestamps)),
	}

	// Extract labels from the value field (second field typically has labels)
	valueField := frame.Schema.Fields[1]
	if valueField.Labels != nil {
		for k, v := range valueField.Labels {
			series.Labels[k] = v
		}
	}

	// Extract unit from field config if present
	if valueField.Config != nil && valueField.Config.Unit != "" {
		series.Unit = valueField.Config.Unit
	}

	// Convert data points
	for i := 0; i < len(timestamps) && i < len(values); i++ {
		ts := extractTimestamp(timestamps[i])
		val := extractFloat64(values[i])

		series.Values = append(series.Values, DataPoint{
			Timestamp: ts,
			Value:     val,
		})
	}

	return series
}

// extractTimestamp converts a timestamp value to ISO8601 format.
// Handles epoch milliseconds (float64 or int64).
func extractTimestamp(v interface{}) string {
	switch ts := v.(type) {
	case float64:
		// Grafana returns timestamps as milliseconds
		sec := int64(ts / 1000)
		nsec := int64((ts - float64(sec*1000)) * 1e6)
		return time.Unix(sec, nsec).UTC().Format(time.RFC3339)
	case int64:
		return time.UnixMilli(ts).UTC().Format(time.RFC3339)
	case json.Number:
		if f, err := ts.Float64(); err == nil {
			sec := int64(f / 1000)
			return time.Unix(sec, 0).UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// extractFloat64 converts a value to float64.
func extractFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f
		}
	}
	return 0
}
