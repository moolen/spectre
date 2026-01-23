package grafana

import "time"

// Baseline represents statistical baseline for a metric
type Baseline struct {
	MetricName  string
	Mean        float64
	StdDev      float64
	SampleCount int
	WindowHour  int
	DayType     string // "weekday" or "weekend"
}

// MetricAnomaly represents a detected anomaly in a metric
type MetricAnomaly struct {
	MetricName string
	Value      float64
	Baseline   float64
	ZScore     float64
	Severity   string // "info", "warning", "critical"
	Timestamp  time.Time
}
