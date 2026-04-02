package embeddedstore

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	queryResultSuccess = "success"
	queryResultError   = "error"

	queryFamilyResourceEvents  = "resource_events"
	queryFamilyExportTimeRange = "export_time_range"
	queryFamilyDistinctMeta    = "distinct_metadata"

	storeMixProjectionOnly = "projection_only"
	storeMixHotOnly        = "hot_only"
	storeMixColdOnly       = "cold_only"
	storeMixMixed          = "mixed"
)

type Metrics struct {
	IngestBatches  prometheus.Counter
	IngestEvents   prometheus.Counter
	IngestErrors   prometheus.Counter
	IngestDuration prometheus.Histogram

	HotEvents    prometheus.Gauge
	HotEvictions *prometheus.CounterVec

	FlushTotal    prometheus.Counter
	FlushErrors   prometheus.Counter
	FlushDuration prometheus.Histogram
	FlushEvents   prometheus.Counter
	FlushBytes    prometheus.Counter

	ActiveSegments prometheus.Gauge

	CheckpointTotal    prometheus.Counter
	CheckpointErrors   prometheus.Counter
	CheckpointDuration prometheus.Histogram

	CompactionTotal    prometheus.Counter
	CompactionErrors   prometheus.Counter
	CompactionDuration prometheus.Histogram

	QueryTotal     *prometheus.CounterVec
	QueryDuration  *prometheus.HistogramVec
	SegmentScans   *prometheus.CounterVec
	HotScans       *prometheus.CounterVec
	UIDDiskLookups *prometheus.CounterVec
	collectors     []prometheus.Collector
	registerer     prometheus.Registerer
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}

	ingestBatches := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_ingest_batches_total",
		Help: "Total number of embedded ingest batches processed.",
	})
	ingestEvents := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_ingest_events_total",
		Help: "Total number of embedded events accepted for ingest.",
	})
	ingestErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_ingest_errors_total",
		Help: "Total number of embedded ingest operations that failed.",
	})
	ingestDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_embedded_ingest_duration_seconds",
		Help:    "Time spent processing embedded ingest batches.",
		Buckets: prometheus.DefBuckets,
	})
	hotEvents := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "spectre_embedded_hot_events",
		Help: "Current number of events retained in the embedded hot store.",
	})
	hotEvictions := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "spectre_embedded_hot_evictions_total",
		Help: "Total number of embedded hot-store evictions by scope.",
	}, []string{"scope"})
	flushTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_flush_total",
		Help: "Total number of embedded flush operations.",
	})
	flushErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_flush_errors_total",
		Help: "Total number of embedded flush operations that failed.",
	})
	flushDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_embedded_flush_duration_seconds",
		Help:    "Time spent flushing embedded hot data to disk.",
		Buckets: prometheus.DefBuckets,
	})
	flushEvents := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_flush_events_total",
		Help: "Total number of events written during embedded flushes.",
	})
	flushBytes := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_flush_bytes_total",
		Help: "Total estimated bytes written during embedded flushes.",
	})
	activeSegments := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "spectre_embedded_active_segments",
		Help: "Current number of active embedded cold segments.",
	})
	checkpointTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_checkpoint_total",
		Help: "Total number of embedded checkpoint operations.",
	})
	checkpointErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_checkpoint_errors_total",
		Help: "Total number of embedded checkpoint operations that failed.",
	})
	checkpointDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_embedded_checkpoint_duration_seconds",
		Help:    "Time spent writing embedded checkpoints.",
		Buckets: prometheus.DefBuckets,
	})
	compactionTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_compaction_total",
		Help: "Total number of embedded compaction operations.",
	})
	compactionErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_embedded_compaction_errors_total",
		Help: "Total number of embedded compaction operations that failed.",
	})
	compactionDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_embedded_compaction_duration_seconds",
		Help:    "Time spent compacting embedded segments.",
		Buckets: prometheus.DefBuckets,
	})
	queryTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "spectre_embedded_query_total",
		Help: "Total number of embedded queries by family, store mix, and result.",
	}, []string{"query_family", "store_mix", "result"})
	queryDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "spectre_embedded_query_duration_seconds",
		Help:    "Time spent serving embedded queries by family, store mix, and result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"query_family", "store_mix", "result"})
	segmentScans := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "spectre_embedded_segment_scans_total",
		Help: "Total number of embedded cold-segment scans by query family.",
	}, []string{"query_family"})
	hotScans := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "spectre_embedded_hot_scans_total",
		Help: "Total number of embedded hot-store scans by query family.",
	}, []string{"query_family"})
	uidDiskLookups := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "spectre_embedded_uid_disk_lookups_total",
		Help: "Total number of embedded cold UID lookups by query family.",
	}, []string{"query_family"})

	collectors := []prometheus.Collector{
		ingestBatches,
		ingestEvents,
		ingestErrors,
		ingestDuration,
		hotEvents,
		hotEvictions,
		flushTotal,
		flushErrors,
		flushDuration,
		flushEvents,
		flushBytes,
		activeSegments,
		checkpointTotal,
		checkpointErrors,
		checkpointDuration,
		compactionTotal,
		compactionErrors,
		compactionDuration,
		queryTotal,
		queryDuration,
		segmentScans,
		hotScans,
		uidDiskLookups,
	}
	reg.MustRegister(collectors...)
	initializeMetricsLabelSets(queryTotal, queryDuration, hotEvictions, segmentScans, hotScans, uidDiskLookups)

	return &Metrics{
		IngestBatches:      ingestBatches,
		IngestEvents:       ingestEvents,
		IngestErrors:       ingestErrors,
		IngestDuration:     ingestDuration,
		HotEvents:          hotEvents,
		HotEvictions:       hotEvictions,
		FlushTotal:         flushTotal,
		FlushErrors:        flushErrors,
		FlushDuration:      flushDuration,
		FlushEvents:        flushEvents,
		FlushBytes:         flushBytes,
		ActiveSegments:     activeSegments,
		CheckpointTotal:    checkpointTotal,
		CheckpointErrors:   checkpointErrors,
		CheckpointDuration: checkpointDuration,
		CompactionTotal:    compactionTotal,
		CompactionErrors:   compactionErrors,
		CompactionDuration: compactionDuration,
		QueryTotal:         queryTotal,
		QueryDuration:      queryDuration,
		SegmentScans:       segmentScans,
		HotScans:           hotScans,
		UIDDiskLookups:     uidDiskLookups,
		collectors:         collectors,
		registerer:         reg,
	}
}

func initializeMetricsLabelSets(
	queryTotal *prometheus.CounterVec,
	queryDuration *prometheus.HistogramVec,
	hotEvictions *prometheus.CounterVec,
	segmentScans *prometheus.CounterVec,
	hotScans *prometheus.CounterVec,
	uidDiskLookups *prometheus.CounterVec,
) {
	queryFamilies := []string{queryFamilyResourceEvents, queryFamilyExportTimeRange, queryFamilyDistinctMeta}
	storeMixes := []string{storeMixProjectionOnly, storeMixHotOnly, storeMixColdOnly, storeMixMixed}
	results := []string{queryResultSuccess, queryResultError}

	for _, family := range queryFamilies {
		for _, mix := range storeMixes {
			for _, result := range results {
				queryTotal.WithLabelValues(family, mix, result)
				queryDuration.WithLabelValues(family, mix, result)
			}
		}
		segmentScans.WithLabelValues(family)
		hotScans.WithLabelValues(family)
		uidDiskLookups.WithLabelValues(family)
	}

	for _, scope := range []string{"global", "uid"} {
		hotEvictions.WithLabelValues(scope)
	}
}

func (m *Metrics) Unregister() {
	if m == nil || m.registerer == nil {
		return
	}
	for _, collector := range m.collectors {
		m.registerer.Unregister(collector)
	}
}

func (m *Metrics) RecordIngest(eventCount int, duration time.Duration, err error) {
	if m == nil {
		return
	}
	m.IngestBatches.Inc()
	if eventCount > 0 {
		m.IngestEvents.Add(float64(eventCount))
	}
	if err != nil {
		m.IngestErrors.Inc()
	}
	m.IngestDuration.Observe(duration.Seconds())
}

func (m *Metrics) SetHotEvents(count int) {
	if m == nil {
		return
	}
	m.HotEvents.Set(float64(count))
}

func (m *Metrics) RecordHotEvictions(scope string, count int) {
	if m == nil || count <= 0 {
		return
	}
	m.HotEvictions.WithLabelValues(scope).Add(float64(count))
}

func (m *Metrics) RecordFlush(duration time.Duration, eventCount int, byteCount int64, err error) {
	if m == nil {
		return
	}
	m.FlushTotal.Inc()
	if err != nil {
		m.FlushErrors.Inc()
	}
	if eventCount > 0 {
		m.FlushEvents.Add(float64(eventCount))
	}
	if byteCount > 0 {
		m.FlushBytes.Add(float64(byteCount))
	}
	m.FlushDuration.Observe(duration.Seconds())
}

func (m *Metrics) SetActiveSegments(count int) {
	if m == nil {
		return
	}
	m.ActiveSegments.Set(float64(count))
}

func (m *Metrics) RecordCheckpoint(duration time.Duration, err error) {
	if m == nil {
		return
	}
	m.CheckpointTotal.Inc()
	if err != nil {
		m.CheckpointErrors.Inc()
	}
	m.CheckpointDuration.Observe(duration.Seconds())
}

func (m *Metrics) RecordCompaction(duration time.Duration, err error) {
	if m == nil {
		return
	}
	m.CompactionTotal.Inc()
	if err != nil {
		m.CompactionErrors.Inc()
	}
	m.CompactionDuration.Observe(duration.Seconds())
}

func (m *Metrics) RecordQuery(queryFamily, storeMix string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	result := queryResultSuccess
	if err != nil {
		result = queryResultError
	}
	m.QueryTotal.WithLabelValues(queryFamily, storeMix, result).Inc()
	m.QueryDuration.WithLabelValues(queryFamily, storeMix, result).Observe(duration.Seconds())
}

func (m *Metrics) RecordSegmentScans(queryFamily string, count int) {
	if m == nil || count <= 0 {
		return
	}
	m.SegmentScans.WithLabelValues(queryFamily).Add(float64(count))
}

func (m *Metrics) RecordHotScans(queryFamily string, count int) {
	if m == nil || count <= 0 {
		return
	}
	m.HotScans.WithLabelValues(queryFamily).Add(float64(count))
}

func (m *Metrics) RecordUIDDiskLookups(queryFamily string, count int) {
	if m == nil || count <= 0 {
		return
	}
	m.UIDDiskLookups.WithLabelValues(queryFamily).Add(float64(count))
}
