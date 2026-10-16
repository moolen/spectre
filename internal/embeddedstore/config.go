package embeddedstore

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type EngineConfig struct {
	DataDir                   string
	HotMaxEvents              int
	HotMaxResourceVersions    int
	FlushInterval             time.Duration
	EmbeddedRetentionDays     int
	CheckpointInterval        time.Duration
	CheckpointRetentionCount  int
	CheckpointMaxTailEvents   int
	CheckpointMaxTailBytes    int64
	CheckpointOnShutdown      bool
	SegmentTargetBytes        int64
	CompactionMinSegments     int
	DisableAutoCompaction     bool
	MetricsRegisterer         prometheus.Registerer
	ProjectionHistoryFallback bool
	// DropUnchangedEvents discards Create/Update events whose payload repeats the
	// object's latest stored state (watch re-lists); see dropUnchangedEvents.
	DropUnchangedEvents bool
	// K8sEventInfoHorizon bounds how long core/v1 Event summaries stay in memory; older
	// ones are read from segments. Zero keeps them for the whole retention window.
	K8sEventInfoHorizon time.Duration
}
