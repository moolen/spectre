package embeddedstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManifestStore_LoadOrCreateManifestCreatesInitialManifest(t *testing.T) {
	dir := t.TempDir()

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, storageFormatVersion, manifest.FormatVersion)
	require.Empty(t, manifest.ActiveSegments)
	require.Empty(t, manifest.Checkpoints)
}

func TestManifestStore_StoreManifestAtomicallyReplacesManifest(t *testing.T) {
	dir := t.TempDir()

	_, err := loadOrCreateManifest(dir)
	require.NoError(t, err)

	err = storeManifest(dir, Manifest{
		FormatVersion: storageFormatVersion,
		ActiveSegments: []SegmentMeta{
			{ID: "seg-001"},
		},
		Checkpoints: []CheckpointMeta{},
	})
	require.NoError(t, err)

	reloaded, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Len(t, reloaded.ActiveSegments, 1)
	require.Equal(t, "seg-001", reloaded.ActiveSegments[0].ID)
}

func TestManifestStore_LoadOrCreateManifestRejectsVersionMismatch(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, manifestFileName)
	err := os.WriteFile(manifestPath, []byte(`{"format_version":2,"active_segments":[],"checkpoints":[]}`), 0o600)
	require.NoError(t, err)

	_, err = loadOrCreateManifest(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported manifest format version")
}

func TestManifestStore_StoreManifestRejectsVersionMismatch(t *testing.T) {
	dir := t.TempDir()

	err := storeManifest(dir, Manifest{
		FormatVersion: storageFormatVersion + 1,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported manifest format version")
}

func TestManifest_LoadLegacyManifestWithoutTailMetadata(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, manifestFileName)
	require.NoError(t, os.WriteFile(manifestPath, []byte(
		`{"format_version":1,"active_segments":[{"id":"seg-001","high_water_mark":42}],"checkpoints":[{"id":"chk-00000000000000000042-1","high_water_mark":42}]}`,
	), 0o600))

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, uint64(42), manifest.ActiveCheckpoint.HighWaterMark)
	require.Equal(t, uint64(42), manifest.ActiveTail.BaseHighWaterMark)
}

func TestManifest_LoadLegacyManifestReconcilesCheckpointMetadataToActiveState(t *testing.T) {
	dir := t.TempDir()
	checkpointID := "chk-00000000000000000077-1"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, checkpointsDirName, checkpointID), 0o755))

	manifestPath := filepath.Join(dir, manifestFileName)
	require.NoError(t, os.WriteFile(manifestPath, []byte(
		`{"format_version":1,"active_segments":[],"checkpoints":[]}`,
	), 0o600))

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, checkpointID, manifest.ActiveCheckpoint.ID)
	require.Equal(t, uint64(77), manifest.ActiveCheckpoint.HighWaterMark)
	require.Equal(t, uint64(77), manifest.ActiveTail.BaseHighWaterMark)
	require.Equal(t, uint64(77), manifest.ActiveTail.LastHighWaterMark)
}

func TestConfig_EffectiveEngineConfigAppliesTailDefaults(t *testing.T) {
	cfg, err := Config{DataDir: t.TempDir()}.EffectiveEngineConfig()
	require.NoError(t, err)
	require.Equal(t, 2048, cfg.CheckpointMaxTailEvents)
	require.Equal(t, int64(16<<20), cfg.CheckpointMaxTailBytes)
	require.True(t, cfg.CheckpointOnShutdown)
	require.Equal(t, defaultCheckpointInterval, cfg.CheckpointInterval)
	require.Equal(t, 30*time.Second, cfg.FlushInterval)
}

func TestConfig_EffectiveEngineConfigPreservesExplicitCheckpointOnShutdownFalse(t *testing.T) {
	cfg, err := Config{
		DataDir:                 t.TempDir(),
		CheckpointOnShutdown:    false,
		CheckpointOnShutdownSet: true,
	}.EffectiveEngineConfig()
	require.NoError(t, err)
	require.False(t, cfg.CheckpointOnShutdown)
}
