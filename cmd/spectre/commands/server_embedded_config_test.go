package commands

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/embeddedstore"
)

func TestDescribeEmbeddedEngineConfig_ExplicitCheckpointMode(t *testing.T) {
	description := describeEmbeddedEngineConfig(embeddedstore.EngineConfig{
		HotMaxEvents:             50000,
		HotMaxResourceVersions:   32,
		FlushInterval:            30 * time.Second,
		CheckpointInterval:       0,
		CheckpointRetentionCount: 3,
		CheckpointMaxTailEvents:  2048,
		CheckpointMaxTailBytes:   16 << 20,
		CheckpointOnShutdown:     true,
		SegmentTargetBytes:       16 << 20,
		CompactionMinSegments:    4,
		EmbeddedRetentionDays:    0,
	})

	expected := "hot_max_events=50000 hot_max_resource_versions=32 flush_interval=30s checkpoint_strategy=explicit+shutdown checkpoint_retention_count=3 checkpoint_max_tail_events=2048 checkpoint_max_tail_bytes=16777216 checkpoint_on_shutdown=true segment_target_bytes=16777216 compaction_min_segments=4 auto_compaction=true retention_days=0"
	if description != expected {
		t.Fatalf("unexpected description:\nwant: %s\ngot:  %s", expected, description)
	}
}

func TestDescribeEmbeddedEngineConfig_PeriodicCheckpointMode(t *testing.T) {
	description := describeEmbeddedEngineConfig(embeddedstore.EngineConfig{
		HotMaxEvents:             100,
		HotMaxResourceVersions:   8,
		FlushInterval:            15 * time.Second,
		CheckpointInterval:       2 * time.Minute,
		CheckpointRetentionCount: 5,
		CheckpointMaxTailEvents:  128,
		CheckpointMaxTailBytes:   4096,
		CheckpointOnShutdown:     false,
		SegmentTargetBytes:       1024,
		CompactionMinSegments:    3,
		DisableAutoCompaction:    true,
		EmbeddedRetentionDays:    7,
	})

	expected := "hot_max_events=100 hot_max_resource_versions=8 flush_interval=15s checkpoint_interval=2m0s checkpoint_retention_count=5 checkpoint_max_tail_events=128 checkpoint_max_tail_bytes=4096 checkpoint_on_shutdown=false segment_target_bytes=1024 compaction_min_segments=3 auto_compaction=false retention_days=7"
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
	previousCheckpointRetentionCount := embeddedCheckpointRetentionCount
	previousCheckpointRetentionCountSet := embeddedCheckpointRetentionCountSet
	previousCheckpointMaxTailEvents := embeddedCheckpointMaxTailEvents
	previousCheckpointMaxTailBytes := embeddedCheckpointMaxTailBytes
	previousCheckpointOnShutdown := embeddedCheckpointOnShutdown
	previousCheckpointOnShutdownSet := embeddedCheckpointOnShutdownSet
	checkpointRetentionCountFlag := serverCmd.Flags().Lookup("embedded-checkpoint-retention-count")
	if checkpointRetentionCountFlag == nil {
		t.Fatal("embedded-checkpoint-retention-count flag not found")
	}
	previousCheckpointRetentionCountFlagValue := checkpointRetentionCountFlag.Value.String()
	previousCheckpointRetentionCountFlagChanged := checkpointRetentionCountFlag.Changed
	checkpointOnShutdownFlag := serverCmd.Flags().Lookup("embedded-checkpoint-on-shutdown")
	if checkpointOnShutdownFlag == nil {
		t.Fatal("embedded-checkpoint-on-shutdown flag not found")
	}
	previousCheckpointOnShutdownFlagValue := checkpointOnShutdownFlag.Value.String()
	previousCheckpointOnShutdownFlagChanged := checkpointOnShutdownFlag.Changed
	t.Cleanup(func() {
		dataDir = previousDataDir
		embeddedCheckpointInterval = previousCheckpointInterval
		embeddedCheckpointRetentionCount = previousCheckpointRetentionCount
		embeddedCheckpointRetentionCountSet = previousCheckpointRetentionCountSet
		embeddedCheckpointMaxTailEvents = previousCheckpointMaxTailEvents
		embeddedCheckpointMaxTailBytes = previousCheckpointMaxTailBytes
		embeddedCheckpointOnShutdown = previousCheckpointOnShutdown
		embeddedCheckpointOnShutdownSet = previousCheckpointOnShutdownSet
		if err := serverCmd.Flags().Set("embedded-checkpoint-retention-count", previousCheckpointRetentionCountFlagValue); err != nil {
			t.Fatalf("restore embedded-checkpoint-retention-count flag: %v", err)
		}
		checkpointRetentionCountFlag.Changed = previousCheckpointRetentionCountFlagChanged
		if err := serverCmd.Flags().Set("embedded-checkpoint-on-shutdown", previousCheckpointOnShutdownFlagValue); err != nil {
			t.Fatalf("restore embedded-checkpoint-on-shutdown flag: %v", err)
		}
		checkpointOnShutdownFlag.Changed = previousCheckpointOnShutdownFlagChanged
		syncEmbeddedCheckpointOnShutdownFlagState(serverCmd)
	})

	dataDir = "/tmp/spectre"
	embeddedCheckpointInterval = 15 * time.Minute
	embeddedCheckpointMaxTailEvents = 128
	embeddedCheckpointMaxTailBytes = 1 << 20
	if err := serverCmd.Flags().Set("embedded-checkpoint-retention-count", "7"); err != nil {
		t.Fatalf("set embedded-checkpoint-retention-count flag: %v", err)
	}
	if err := serverCmd.Flags().Set("embedded-checkpoint-on-shutdown", "false"); err != nil {
		t.Fatalf("set embedded-checkpoint-on-shutdown flag: %v", err)
	}
	syncEmbeddedCheckpointOnShutdownFlagState(serverCmd)

	cfg := embeddedStoreConfig()
	if cfg.DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.CheckpointInterval != 15*time.Minute {
		t.Fatalf("expected checkpoint interval 15m, got %s", cfg.CheckpointInterval)
	}
	if cfg.CheckpointRetentionCount != 7 {
		t.Fatalf("expected checkpoint retention count 7, got %d", cfg.CheckpointRetentionCount)
	}
	if !cfg.CheckpointRetentionCountSet {
		t.Fatal("expected checkpoint retention override to be marked as explicitly set")
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

func TestEmbeddedStoreConfigPreservesExplicitCheckpointRetentionDisable(t *testing.T) {
	previousCheckpointRetentionCount := embeddedCheckpointRetentionCount
	previousCheckpointRetentionCountSet := embeddedCheckpointRetentionCountSet
	checkpointRetentionCountFlag := serverCmd.Flags().Lookup("embedded-checkpoint-retention-count")
	if checkpointRetentionCountFlag == nil {
		t.Fatal("embedded-checkpoint-retention-count flag not found")
	}
	previousCheckpointRetentionCountFlagValue := checkpointRetentionCountFlag.Value.String()
	previousCheckpointRetentionCountFlagChanged := checkpointRetentionCountFlag.Changed
	t.Cleanup(func() {
		embeddedCheckpointRetentionCount = previousCheckpointRetentionCount
		embeddedCheckpointRetentionCountSet = previousCheckpointRetentionCountSet
		if err := serverCmd.Flags().Set("embedded-checkpoint-retention-count", previousCheckpointRetentionCountFlagValue); err != nil {
			t.Fatalf("restore embedded-checkpoint-retention-count flag: %v", err)
		}
		checkpointRetentionCountFlag.Changed = previousCheckpointRetentionCountFlagChanged
		syncEmbeddedCheckpointOnShutdownFlagState(serverCmd)
	})

	if err := serverCmd.Flags().Set("embedded-checkpoint-retention-count", "0"); err != nil {
		t.Fatalf("set embedded-checkpoint-retention-count flag: %v", err)
	}
	syncEmbeddedCheckpointOnShutdownFlagState(serverCmd)

	cfg := embeddedStoreConfig()
	if cfg.CheckpointRetentionCount != 0 {
		t.Fatalf("expected checkpoint retention count 0, got %d", cfg.CheckpointRetentionCount)
	}
	if !cfg.CheckpointRetentionCountSet {
		t.Fatal("expected checkpoint retention disable to be marked as explicitly set")
	}
}

func TestEmbeddedStoreConfigMarksCheckpointOnShutdownAsExplicitWhenSetTrue(t *testing.T) {
	previousCheckpointOnShutdown := embeddedCheckpointOnShutdown
	previousCheckpointOnShutdownSet := embeddedCheckpointOnShutdownSet
	checkpointOnShutdownFlag := serverCmd.Flags().Lookup("embedded-checkpoint-on-shutdown")
	if checkpointOnShutdownFlag == nil {
		t.Fatal("embedded-checkpoint-on-shutdown flag not found")
	}
	previousCheckpointOnShutdownFlagValue := checkpointOnShutdownFlag.Value.String()
	previousCheckpointOnShutdownFlagChanged := checkpointOnShutdownFlag.Changed
	t.Cleanup(func() {
		embeddedCheckpointOnShutdown = previousCheckpointOnShutdown
		embeddedCheckpointOnShutdownSet = previousCheckpointOnShutdownSet
		if err := serverCmd.Flags().Set("embedded-checkpoint-on-shutdown", previousCheckpointOnShutdownFlagValue); err != nil {
			t.Fatalf("restore embedded-checkpoint-on-shutdown flag: %v", err)
		}
		checkpointOnShutdownFlag.Changed = previousCheckpointOnShutdownFlagChanged
		syncEmbeddedCheckpointOnShutdownFlagState(serverCmd)
	})

	if err := serverCmd.Flags().Set("embedded-checkpoint-on-shutdown", "true"); err != nil {
		t.Fatalf("set embedded-checkpoint-on-shutdown flag: %v", err)
	}
	syncEmbeddedCheckpointOnShutdownFlagState(serverCmd)

	cfg := embeddedStoreConfig()
	if !cfg.CheckpointOnShutdown {
		t.Fatal("expected checkpoint on shutdown to be true")
	}
	if !cfg.CheckpointOnShutdownSet {
		t.Fatal("expected checkpoint on shutdown override to be marked as explicitly set")
	}
}

func TestEmbeddedStoreConfigIncludesFlushAndSegmentOverrides(t *testing.T) {
	previousFlushInterval := embeddedFlushInterval
	previousSegmentTargetBytes := embeddedSegmentTargetBytes
	previousCompactionMinSegments := embeddedCompactionMinSegments
	previousAutoCompaction := embeddedAutoCompaction
	previousEmbeddedRetentionDays := embeddedRetentionDays
	flushIntervalFlag := serverCmd.Flags().Lookup("embedded-flush-interval")
	if flushIntervalFlag == nil {
		t.Fatal("embedded-flush-interval flag not found")
	}
	previousFlushIntervalFlagValue := flushIntervalFlag.Value.String()
	segmentTargetBytesFlag := serverCmd.Flags().Lookup("embedded-segment-target-bytes")
	if segmentTargetBytesFlag == nil {
		t.Fatal("embedded-segment-target-bytes flag not found")
	}
	previousSegmentTargetBytesFlagValue := segmentTargetBytesFlag.Value.String()
	compactionMinSegmentsFlag := serverCmd.Flags().Lookup("embedded-compaction-min-segments")
	if compactionMinSegmentsFlag == nil {
		t.Fatal("embedded-compaction-min-segments flag not found")
	}
	previousCompactionMinSegmentsFlagValue := compactionMinSegmentsFlag.Value.String()
	autoCompactionFlag := serverCmd.Flags().Lookup("embedded-auto-compaction")
	if autoCompactionFlag == nil {
		t.Fatal("embedded-auto-compaction flag not found")
	}
	previousAutoCompactionFlagValue := autoCompactionFlag.Value.String()
	retentionDaysFlag := serverCmd.Flags().Lookup("embedded-retention-days")
	if retentionDaysFlag == nil {
		t.Fatal("embedded-retention-days flag not found")
	}
	previousRetentionDaysFlagValue := retentionDaysFlag.Value.String()
	t.Cleanup(func() {
		embeddedFlushInterval = previousFlushInterval
		embeddedSegmentTargetBytes = previousSegmentTargetBytes
		embeddedCompactionMinSegments = previousCompactionMinSegments
		embeddedAutoCompaction = previousAutoCompaction
		embeddedRetentionDays = previousEmbeddedRetentionDays
		requireNoError(t, serverCmd.Flags().Set("embedded-flush-interval", previousFlushIntervalFlagValue))
		requireNoError(t, serverCmd.Flags().Set("embedded-segment-target-bytes", previousSegmentTargetBytesFlagValue))
		requireNoError(t, serverCmd.Flags().Set("embedded-compaction-min-segments", previousCompactionMinSegmentsFlagValue))
		requireNoError(t, serverCmd.Flags().Set("embedded-auto-compaction", previousAutoCompactionFlagValue))
		requireNoError(t, serverCmd.Flags().Set("embedded-retention-days", previousRetentionDaysFlagValue))
	})

	requireNoError(t, serverCmd.Flags().Set("embedded-flush-interval", "2m"))
	requireNoError(t, serverCmd.Flags().Set("embedded-segment-target-bytes", "33554432"))
	requireNoError(t, serverCmd.Flags().Set("embedded-compaction-min-segments", "8"))
	requireNoError(t, serverCmd.Flags().Set("embedded-auto-compaction", "false"))
	requireNoError(t, serverCmd.Flags().Set("embedded-retention-days", "14"))

	cfg := embeddedStoreConfig()
	if cfg.FlushInterval != 2*time.Minute {
		t.Fatalf("expected flush interval 2m, got %s", cfg.FlushInterval)
	}
	if cfg.SegmentTargetBytes != 33554432 {
		t.Fatalf("expected segment target bytes 33554432, got %d", cfg.SegmentTargetBytes)
	}
	if cfg.CompactionMinSegments != 8 {
		t.Fatalf("expected compaction min segments 8, got %d", cfg.CompactionMinSegments)
	}
	if !cfg.DisableAutoCompaction {
		t.Fatal("expected auto compaction to be disabled when flag is false")
	}
	if cfg.EmbeddedRetentionDays != 14 {
		t.Fatalf("expected embedded retention days 14, got %d", cfg.EmbeddedRetentionDays)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
