package commands

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/embeddedstore"
)

func TestDescribeEmbeddedEngineConfig_ExplicitCheckpointMode(t *testing.T) {
	description := describeEmbeddedEngineConfig(embeddedstore.EngineConfig{
		HotMaxEvents:           50000,
		HotMaxResourceVersions: 32,
		FlushInterval:          30 * time.Second,
		CheckpointInterval:     0,
		SegmentTargetBytes:     16 << 20,
		CompactionMinSegments:  4,
	})

	expected := "hot_max_events=50000 hot_max_resource_versions=32 flush_interval=30s checkpoint_strategy=explicit+shutdown segment_target_bytes=16777216 compaction_min_segments=4"
	if description != expected {
		t.Fatalf("unexpected description:\nwant: %s\ngot:  %s", expected, description)
	}
}

func TestDescribeEmbeddedEngineConfig_PeriodicCheckpointMode(t *testing.T) {
	description := describeEmbeddedEngineConfig(embeddedstore.EngineConfig{
		HotMaxEvents:           100,
		HotMaxResourceVersions: 8,
		FlushInterval:          15 * time.Second,
		CheckpointInterval:     2 * time.Minute,
		SegmentTargetBytes:     1024,
		CompactionMinSegments:  3,
	})

	expected := "hot_max_events=100 hot_max_resource_versions=8 flush_interval=15s checkpoint_interval=2m0s segment_target_bytes=1024 compaction_min_segments=3"
	if description != expected {
		t.Fatalf("unexpected description:\nwant: %s\ngot:  %s", expected, description)
	}
}

func TestEmbeddedImportAPIEnabled(t *testing.T) {
	if !embeddedImportAPIEnabled(serverRuntimeMode{Embedded: true, StartWatcher: true}) {
		t.Fatal("expected live embedded mode to expose import API")
	}
	if embeddedImportAPIEnabled(serverRuntimeMode{Embedded: true, StartWatcher: false, ImportOnly: true}) {
		t.Fatal("expected read-only embedded mode to disable import API")
	}
	if embeddedImportAPIEnabled(serverRuntimeMode{Embedded: false, StartWatcher: true}) {
		t.Fatal("expected non-embedded mode helper to stay false")
	}
}

func TestEmbeddedStoreConfigIncludesCheckpointIntervalOverride(t *testing.T) {
	previousDataDir := dataDir
	previousCheckpointInterval := embeddedCheckpointInterval
	t.Cleanup(func() {
		dataDir = previousDataDir
		embeddedCheckpointInterval = previousCheckpointInterval
	})

	dataDir = "/tmp/spectre"
	embeddedCheckpointInterval = 15 * time.Minute

	cfg := embeddedStoreConfig()
	if cfg.DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.CheckpointInterval != 15*time.Minute {
		t.Fatalf("expected checkpoint interval 15m, got %s", cfg.CheckpointInterval)
	}
}
