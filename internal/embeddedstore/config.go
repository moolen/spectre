package embeddedstore

import "time"

type EngineConfig struct {
	DataDir                string
	HotMaxEvents           int
	HotMaxResourceVersions int
	FlushInterval          time.Duration
	CheckpointInterval     time.Duration
	SegmentTargetBytes     int64
	CompactionMinSegments  int
}
