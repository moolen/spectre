package commands

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/embeddedstore"
)

func TestDescribeEmbeddedEngineConfig_ExplicitCheckpointMode(t *testing.T) {
	description := describeEmbeddedEngineConfig(embeddedstore.EngineConfig{
		HotMaxEvents:            50000,
		HotMaxResourceVersions:  32,
		FlushInterval:           30 * time.Second,
		CheckpointInterval:      0,
		CheckpointMaxTailEvents: 2048,
		CheckpointMaxTailBytes:  16 << 20,
		CheckpointOnShutdown:    true,
		SegmentTargetBytes:      16 << 20,
		CompactionMinSegments:   4,
	})

	expected := "hot_max_events=50000 hot_max_resource_versions=32 flush_interval=30s checkpoint_strategy=explicit+shutdown checkpoint_max_tail_events=2048 checkpoint_max_tail_bytes=16777216 checkpoint_on_shutdown=true segment_target_bytes=16777216 compaction_min_segments=4"
	if description != expected {
		t.Fatalf("unexpected description:\nwant: %s\ngot:  %s", expected, description)
	}
}

func TestDescribeEmbeddedEngineConfig_PeriodicCheckpointMode(t *testing.T) {
	description := describeEmbeddedEngineConfig(embeddedstore.EngineConfig{
		HotMaxEvents:            100,
		HotMaxResourceVersions:  8,
		FlushInterval:           15 * time.Second,
		CheckpointInterval:      2 * time.Minute,
		CheckpointMaxTailEvents: 128,
		CheckpointMaxTailBytes:  4096,
		CheckpointOnShutdown:    false,
		SegmentTargetBytes:      1024,
		CompactionMinSegments:   3,
	})

	expected := "hot_max_events=100 hot_max_resource_versions=8 flush_interval=15s checkpoint_interval=2m0s checkpoint_max_tail_events=128 checkpoint_max_tail_bytes=4096 checkpoint_on_shutdown=false segment_target_bytes=1024 compaction_min_segments=3"
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
	previousCheckpointMaxTailEvents := embeddedCheckpointMaxTailEvents
	previousCheckpointMaxTailBytes := embeddedCheckpointMaxTailBytes
	previousCheckpointOnShutdown := embeddedCheckpointOnShutdown
	checkpointOnShutdownFlag := serverCmd.Flags().Lookup("embedded-checkpoint-on-shutdown")
	if checkpointOnShutdownFlag == nil {
		t.Fatal("embedded-checkpoint-on-shutdown flag not found")
	}
	previousCheckpointOnShutdownFlagValue := checkpointOnShutdownFlag.Value.String()
	previousCheckpointOnShutdownFlagChanged := checkpointOnShutdownFlag.Changed
	t.Cleanup(func() {
		dataDir = previousDataDir
		embeddedCheckpointInterval = previousCheckpointInterval
		embeddedCheckpointMaxTailEvents = previousCheckpointMaxTailEvents
		embeddedCheckpointMaxTailBytes = previousCheckpointMaxTailBytes
		embeddedCheckpointOnShutdown = previousCheckpointOnShutdown
		if err := serverCmd.Flags().Set("embedded-checkpoint-on-shutdown", previousCheckpointOnShutdownFlagValue); err != nil {
			t.Fatalf("restore embedded-checkpoint-on-shutdown flag: %v", err)
		}
		checkpointOnShutdownFlag.Changed = previousCheckpointOnShutdownFlagChanged
	})

	dataDir = "/tmp/spectre"
	embeddedCheckpointInterval = 15 * time.Minute
	embeddedCheckpointMaxTailEvents = 128
	embeddedCheckpointMaxTailBytes = 1 << 20
	if err := serverCmd.Flags().Set("embedded-checkpoint-on-shutdown", "false"); err != nil {
		t.Fatalf("set embedded-checkpoint-on-shutdown flag: %v", err)
	}

	cfg := embeddedStoreConfig()
	if cfg.DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.CheckpointInterval != 15*time.Minute {
		t.Fatalf("expected checkpoint interval 15m, got %s", cfg.CheckpointInterval)
	}
	if cfg.CheckpointMaxTailEvents != 128 {
		t.Fatalf("expected checkpoint max tail events 128, got %d", cfg.CheckpointMaxTailEvents)
	}
	if cfg.CheckpointMaxTailBytes != 1<<20 {
		t.Fatalf("expected checkpoint max tail bytes %d, got %d", 1<<20, cfg.CheckpointMaxTailBytes)
	}
	if cfg.CheckpointOnShutdown {
		t.Fatal("expected checkpoint on shutdown to be false")
	}
	if !cfg.CheckpointOnShutdownSet {
		t.Fatal("expected checkpoint on shutdown override to be marked as explicitly set")
	}
}
