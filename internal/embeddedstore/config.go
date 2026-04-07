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
	CheckpointInterval        time.Duration
	CheckpointRetentionCount  int
	CheckpointMaxTailEvents   int
	CheckpointMaxTailBytes    int64
	CheckpointOnShutdown      bool
	SegmentTargetBytes        int64
	CompactionMinSegments     int
	MetricsRegisterer         prometheus.Registerer
	ProjectionHistoryFallback bool
}
